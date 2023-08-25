// Copyright(C) 2023 TBD
// SPDX-License-Identifier: TBD

package main

import (
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

func main() {
	var logger = log.NewLogfmtLogger(os.Stderr)
	var toolkitFlags = webflag.AddFlags(kingpin.CommandLine, ":8081")
	kingpin.Parse()

	srv := &http.Server{
		ReadTimeout: 1 * time.Second,
	}
	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	accountBalance := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "bank",
			Name:      "account_balance",
			Help:      "This is the amount of money in the bank account",
		})

	http.Handle("/metrics", promhttp.Handler())
	prometheus.MustRegister(accountBalance)

	go func() {
		if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil {
			_ = level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
			close(srvc)
		}
	}()

	go func() {
		for {
			accountBalance.Set(rand.Float64() * 100) //nolint: gosec
			time.Sleep(time.Second)
		}
	}()

	for {
		select {
		case <-term:
			_ = level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			os.Exit(0)
		case <-srvc:
			os.Exit(1)
		}
	}
}
