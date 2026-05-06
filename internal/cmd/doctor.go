package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/lambdabaa/dewey/apps/cli/internal/api"
	"github.com/lambdabaa/dewey/apps/cli/internal/config"
	"github.com/spf13/cobra"
)

func newDoctorCmd(makeAppCtx func(bool) (*AppContext, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the local Dewey CLI environment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := makeAppCtx(false)
			if err != nil {
				return err
			}
			r := app.Renderer
			r.Statusln(r.Bold("Checking environment..."))

			results := []doctorCheck{}

			// 1. API key
			apiKey := app.Client.APIKey
			if apiKey == "" {
				results = append(results, doctorCheck{ok: false, label: "DEWEY_API_KEY is not set", hint: "Set DEWEY_API_KEY before running data-plane commands"})
			} else {
				results = append(results, doctorCheck{ok: true, label: fmt.Sprintf("DEWEY_API_KEY is set (%s)", config.MaskAPIKey(apiKey))})
			}

			// 2. Base URL
			baseURL := app.Client.BaseURL
			defaultLabel := ""
			if baseURL == api.DefaultBaseURL {
				defaultLabel = " (default)"
			}
			results = append(results, doctorCheck{ok: true, label: fmt.Sprintf("Base URL → %s%s", baseURL, defaultLabel)})

			// 3. DNS + 4. TLS handshake
			parsed, err := url.Parse(baseURL)
			if err != nil {
				results = append(results, doctorCheck{ok: false, label: "Base URL is malformed", hint: err.Error()})
			} else {
				host := parsed.Host
				if !strings.Contains(host, ":") {
					if parsed.Scheme == "https" {
						host += ":443"
					} else {
						host += ":80"
					}
				}
				if _, err := net.LookupHost(parsed.Hostname()); err != nil {
					results = append(results, doctorCheck{ok: false, label: "DNS resolves", hint: err.Error()})
				} else {
					results = append(results, doctorCheck{ok: true, label: "DNS resolves"})
				}
				if parsed.Scheme == "https" {
					tlsResult := checkTLS(host)
					results = append(results, tlsResult)
				}
			}

			// 5. Auth + collection count
			if apiKey != "" {
				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()
				ping, err := app.Client.Ping(ctx)
				if err != nil {
					hint := ""
					if apiErr, ok := err.(*api.Error); ok && (apiErr.Status == 401 || apiErr.Status == 403) {
						hint = "API key was rejected — verify it is active and matches the project"
					}
					results = append(results, doctorCheck{ok: false, label: "GET /collections", hint: err.Error() + ifNotEmpty(hint, "; "+hint)})
				} else {
					results = append(results, doctorCheck{
						ok:    true,
						label: fmt.Sprintf("GET /collections returned %d in %s (%d collections)", ping.Status, ping.Latency.Round(time.Millisecond), ping.Collections),
					})
					if ping.Collections == 0 {
						results = append(results, doctorCheck{ok: true, warn: true, label: "No collections yet — try `dewey collections create <name>`"})
					}
				}
			}

			// 6. Telemetry
			if config.TelemetryEnabled() {
				results = append(results, doctorCheck{ok: true, label: "Anonymous telemetry: on (DEWEY_TELEMETRY=0 to opt out)"})
			} else {
				results = append(results, doctorCheck{ok: true, label: "Anonymous telemetry: off"})
			}

			// Render
			anyFail := false
			for _, res := range results {
				icon := r.Tick()
				if !res.ok {
					icon = r.Cross()
					anyFail = true
				} else if res.warn {
					icon = r.Warn()
				}
				r.Println("  ", icon, res.label)
				if res.hint != "" {
					r.Println("     ", r.Dim("hint: "+res.hint))
				}
			}
			if anyFail {
				return fmt.Errorf("doctor: some checks failed")
			}
			r.Println(r.Bold("All systems go."))
			return nil
		},
	}
}

type doctorCheck struct {
	ok    bool
	warn  bool
	label string
	hint  string
}

func checkTLS(hostport string) doctorCheck {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", hostport, &tls.Config{})
	if err != nil {
		return doctorCheck{ok: false, label: "TLS handshake", hint: err.Error()}
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return doctorCheck{ok: false, label: "TLS handshake", hint: "no peer certificates"}
	}
	cert := state.PeerCertificates[0]
	issuer := cert.Issuer.CommonName
	daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
	return doctorCheck{
		ok:    true,
		label: fmt.Sprintf("TLS handshake (issued by %s, valid %d days)", issuer, daysLeft),
	}
}

func ifNotEmpty(s, suffix string) string {
	if s == "" {
		return ""
	}
	return suffix
}
