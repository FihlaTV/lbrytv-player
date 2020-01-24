package player

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const ParamDownload = "download"

// RequestHandler is a HTTP request handler for player package.
type RequestHandler struct {
	player *Player
}

// NewRequestHandler initializes a HTTP request handler with the provided Player instance.
func NewRequestHandler(p *Player) *RequestHandler {
	return &RequestHandler{p}
}

func (h RequestHandler) getURI(r *http.Request) string {
	vars := mux.Vars(r)
	return fmt.Sprintf("%s#%s", vars["uri"], vars["claim"])
}

func (h RequestHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, msg string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(msg))
}

func (h RequestHandler) writeHeaders(w http.ResponseWriter, r *http.Request, s *Stream) {
	header := w.Header()
	header.Set("Content-Length", fmt.Sprintf("%v", s.Size))
	header.Set("Content-Type", s.ContentType)
	header.Set("Last-Modified", s.Timestamp().UTC().Format(http.TimeFormat))
	if r.URL.Query().Get(ParamDownload) != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%v", s.Claim.Value.GetStream().Source.Name))
	}
}

func (h RequestHandler) processStreamError(w http.ResponseWriter, uri string, err error) {
	if errors.Is(err, errPaidStream) {
		h.writeErrorResponse(w, http.StatusPaymentRequired, err.Error())
	} else if errors.Is(err, errStreamNotFound) {
		h.writeErrorResponse(w, http.StatusNotFound, err.Error())
	} else if strings.Contains(err.Error(), "blob not found") {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	} else if strings.Contains(err.Error(), "hash in response does not match") {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, err.Error())
	} else {
		// logger.CaptureException(err, map[string]string{"uri": uri})
		h.writeErrorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// Handle is responsible for all HTTP media delivery via player module.
func (h *RequestHandler) Handle(w http.ResponseWriter, r *http.Request) {
	uri := h.getURI(r)
	Logger.streamPlaybackRequested(uri, "") // , users.GetIPAddressForRequest(r))

	s, err := h.player.ResolveStream(uri)
	if err != nil {
		Logger.streamResolveFailed(uri, err)
		h.processStreamError(w, uri, err)
		return
	}
	Logger.streamResolved(s)

	err = h.player.RetrieveStream(s)
	if err != nil {
		Logger.streamRetrievalFailed(uri, err)
		h.processStreamError(w, uri, err)
		return
	}
	Logger.streamRetrieved(s)

	h.writeHeaders(w, r, s)

	err = h.player.Play(s, w, r)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}
}

// HandleHead handlers OPTIONS requests for media.
func (h *RequestHandler) HandleHead(w http.ResponseWriter, r *http.Request) {
	uri := h.getURI(r)

	s, err := h.player.ResolveStream(uri)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}

	err = h.player.RetrieveStream(s)
	if err != nil {
		h.processStreamError(w, uri, err)
		return
	}

	h.writeHeaders(w, r, s)
	w.WriteHeader(http.StatusOK)
}

func InstallPlayerRoutes(r *mux.Router, p *Player) {
	playerHandler := NewRequestHandler(p)
	playerRouter := r.Path("/content/claims/{uri}/{claim}/{filename}").Subrouter()
	playerRouter.HandleFunc("", playerHandler.Handle).Methods(http.MethodGet)
	playerRouter.HandleFunc("", playerHandler.HandleHead).Methods(http.MethodHead)
}
