package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/povilasv/prommod"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/rs/zerolog"

	"github.com/nextcaller/sip-capture/collect"
	"github.com/nextcaller/sip-capture/defrag"
	"github.com/nextcaller/sip-capture/extract"
	"github.com/nextcaller/sip-capture/filters"
	"github.com/nextcaller/sip-capture/publisher"
	"github.com/nextcaller/sip-capture/source"
)

var (
	// The following vars are meant to be filled in by
	// `go build -ldflags -X=main.<X>=<Value>`.
	// `make sip-capture` should do this for you.

	// Version is the git tag of this build (v1.2.3)
	Version = "unknown"
	// Build is the git short hash ref of this build (123abcdef)
	Build = "unknown"
	// Branch is the git branch for this build (master)
	Branch = "unknown"
	// Date is when this build was created (2020-01-02T03:04:05Z)
	Date = "unknown"
)

func run(args []string, stdout io.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zerolog.New(stdout).With().Timestamp().Str("app", "sip-capture").Logger()
	ctx = log.WithContext(ctx)

	cfg := &config{}
	if err := cfg.Load(args); err != nil {
		return fmt.Errorf("unable to load config: %w", err)
	}

	level, err := zerolog.ParseLevel(strings.ToLower(cfg.LogLevel))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Debug().Msg("debug logging active")

	log.Debug().Msg("setting up signal handling")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() { <-signals; log.Debug().Msg("received quit signal"); cancel() }()

	log.Debug().Msg("compiling SIP selection filter")
	filter, err := filters.Compile(cfg.SIPFilter)
	if err != nil {
		return fmt.Errorf("unable to compile SIP filter: %w", err)
	}

	log.Debug().Msg("creating MQTT publisher")
	publ := publisher.NewMQTT(cfg.MQTT)
	if err := publ.Connect(ctx); err != nil {
		return fmt.Errorf("unable to connect to MQTT broker: %w", err)
	}

	log.Debug().Msg("building message collecter")
	collecter := collect.NewCollecter(filter, publ.Publish, 10000)
	go collecter.Publish(ctx)

	log.Debug().Msg("initializing pcap source")
	capture, err := source.NewPCAP(cfg.Interface, cfg.BPFFilter)
	if err != nil {
		return fmt.Errorf("unable to initialize pcap source: %w", err)
	}

	log.Debug().Msg("launching source shutdown closer")
	go func() { <-ctx.Done(); capture.Close() }()

	log.Debug().Msg("building packet defragmentation assembler")
	defragger := defrag.NewIPv4Defragmenter()

	log.Debug().Msg("building SIP packet message extracter")
	extracter := extract.NewExtracter(defragger)

	if cfg.MetricsAddr != "" {
		log.Debug().Msg("creating Prometheus registry")
		reg := prometheus.NewRegistry()
		version.Version = Version
		version.Revision = Build
		version.Branch = Branch
		version.BuildDate = Date
		reg.MustRegister(
			version.NewCollector("sipcapture"),
			prommod.NewCollector("sipcapture"),
			prometheus.NewGoCollector(),
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		)
		reg.MustRegister(capture.Metrics()...)
		reg.MustRegister(extracter.Metrics()...)
		reg.MustRegister(collecter.Metrics()...)

		log.Debug().
			Str("address", cfg.MetricsAddr).
			Str("path", "/metrics").
			Msg("publishing Prometheus endpoint")
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		srv := &http.Server{Handler: mux, Addr: cfg.MetricsAddr}
		// Since we never call srv.Shutdown(), ListenAndServe will only ever
		// return if the underlying socket fails.
		go log.Err(srv.ListenAndServe()).Msg("http metrics endpoint failed")
	}

	log.Debug().Msg("beginning signaling capture")
	extracter.Extract(ctx, capture.Packets(), collecter.Accept)

	publ.Close()
	log.Info().Msg("shutdown complete.")

	return nil
}

func main() {
	// these are stateful global module level changes; only do them in main
	time.Local = time.UTC
	zerolog.TimeFieldFormat = "2006-01-02T15:04:05.999Z07:00"

	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
