package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

type callbackResult struct {
	code  string
	state string
	err   error
}

func selfSignedTLS() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Traversal"}},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * time.Minute),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  key,
		}},
	}, nil
}

func startCallbackServer(expectedState string) (<-chan callbackResult, *http.Server, error) {
	tlsCfg, err := selfSignedTLS()
	if err != nil {
		return nil, nil, fmt.Errorf("tls setup: %w", err)
	}

	ch := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: ":8080", Handler: mux, TLSConfig: tlsCfg}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errParam := q.Get("error"); errParam != "" {
			fmt.Fprintf(w, "Authentication failed: %s — you can close this window.", errParam)
			ch <- callbackResult{err: fmt.Errorf("sso error: %s", errParam)}
			go srv.Shutdown(context.Background())
			return
		}

		if q.Get("state") != expectedState {
			fmt.Fprintf(w, "Authentication failed: state mismatch — you can close this window.")
			ch <- callbackResult{err: fmt.Errorf("state mismatch: possible CSRF")}
			go srv.Shutdown(context.Background())
			return
		}

		fmt.Fprintf(w, "Authentication successful! You can close this window.")
		ch <- callbackResult{code: q.Get("code"), state: q.Get("state")}
		go srv.Shutdown(context.Background())
	})

	return ch, srv, nil
}
