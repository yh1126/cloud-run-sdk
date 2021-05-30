package sdk

import (
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog"
)

type Adapter func(http.Handler) http.Handler

func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

func InjectLogger(logger zerolog.Logger, debug bool) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			r = r.WithContext(logger.WithContext(r.Context()))

			if isCloudRun() {
				traceID, _ := traceContextFromHeader(r.Header.Get("X-Cloud-Trace-Context"))
				if traceID == "" {
					h.ServeHTTP(w, r)
					return
				}
				trace := fmt.Sprintf("projects/%s/traces/%s", projectID, traceID)

				logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
					return c.Str("logging.googleapis.com/trace", trace)
				})
			}

			h.ServeHTTP(w, r)
		})
	}
}

func RegisterDefaultHTTPServer(fn http.HandlerFunc, adapters ...Adapter) *http.Server {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	hostAddr := "0.0.0.0"
	if h := os.Getenv("HOST_ADDR"); h != "" {
		hostAddr = h
	}

	mux := http.NewServeMux()
	mux.Handle("/", Adapt(fn, adapters...))

	return &http.Server{
		Addr:    fmt.Sprintf("%s:%s", hostAddr, port),
		Handler: mux,
	}
}
