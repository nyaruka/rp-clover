package clover

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/nyaruka/rp-clover/models"
	"github.com/sirupsen/logrus"
)

// handles an interchange request
func handleInterchange(s *Server, w http.ResponseWriter, r *http.Request) error {
	interchangeUUID := chi.URLParam(r, "interchangeUUID")

	// look up our interchange
	interchange, err := models.GetInterchange(r.Context(), s.db, interchangeUUID)
	if err != nil {
		return err
	}

	if interchange == nil {
		return writeErrorResponse(r.Context(), w, http.StatusNotFound, "interchange not found", fmt.Errorf("interchange not found"))
	}

	// get our URN from our incoming message
	err = r.ParseForm()
	if err != nil {
		return err
	}

	sender := r.Form.Get("sender")
	if sender == "" {
		return writeErrorResponse(r.Context(), w, http.StatusBadRequest, "missing sender field", fmt.Errorf("missing sender field"))
	}
	urn := interchange.Scheme + ":" + sender

	// the channel we will route to
	var routedChannel *models.Channel
	var routingReason string

	// get our text
	message := r.Form.Get("message")

	// see if our text is any of our keywords, if so, assign this URN to that channel
	message = strings.TrimSpace(message)
	for _, channel := range interchange.Channels {
		for _, keyword := range channel.Keywords {
			if message == keyword {
				routedChannel = &channel
				routingReason = fmt.Sprintf("keyword '%s'", keyword)
				break
			}
		}

		// we found a matching channel, associate this URN
		if routedChannel != nil {
			err := models.SetChannelForURN(r.Context(), s.db, interchange, routedChannel, urn)
			if err != nil {
				return err
			}
			break
		}
	}

	// if not, look up current mapping for this URN
	if routedChannel == nil {
		routedChannel, err = models.GetChannelForURN(r.Context(), s.db, interchange, urn)
		if err != nil {
			return err
		}

		if routedChannel != nil {
			routingReason = "urn mapping"
		}
	}

	// didn't find any explicit routes, use our default chanel
	if routedChannel == nil {
		routedChannel = &interchange.Channels[0]
		routingReason = "default channel"
	}

	logrus.WithFields(logrus.Fields{
		"interchange_uuid": interchange.UUID,
		"channel_uuid":     routedChannel.UUID,
		"base_url":         routedChannel.URL,
		"urn":              urn,
		"message":          message,
		"routing_reason":   routingReason,
	}).Info("forwarding request")

	return forwardRequest(r.Context(), w, r, interchange, routedChannel)
}

func forwardRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, interchange *models.Interchange, channel *models.Channel) error {
	// parse our channel URL
	queryPart := ""
	if r.URL.RawQuery != "" {
		queryPart = "?" + r.URL.RawQuery
	}
	outURL, err := url.Parse(channel.URL + queryPart)
	if err != nil {
		return err
	}

	// create our new outbound request
	outRequest, err := http.NewRequest(r.Method, outURL.String(), bytes.NewReader([]byte(r.PostForm.Encode())))
	if err != nil {
		return err
	}

	// set any headers
	outRequest.Header = r.Header

	log := logrus.WithFields(logrus.Fields{
		"channel_uuid": channel.UUID,
		"url":          outURL,
		"method":       outRequest.Method,
	})

	if r.Method == http.MethodPost {
		log = log.WithField("form", r.PostForm)
	}

	// fire it off
	resp, err := client.Do(outRequest)
	if err != nil {
		log.WithError(err).Error("error fowarding request")
		return err
	}

	log.WithField("status_code", resp.StatusCode).Info("request forwarded")

	// we respond in the same way our downstream server did
	w.WriteHeader(resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_, err = w.Write(body)

	return err
}

var client *http.Client

func init() {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}
	client = &http.Client{Transport: tr}
}
