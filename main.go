package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/songzhibin97/CryptoPulse/api"
	"github.com/songzhibin97/CryptoPulse/config"
	"github.com/songzhibin97/CryptoPulse/report"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	reportMgr := report.NewReportManager("reports")
	r := gin.Default()
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	api.SetupRoutes(r, cfg, log.Logger, reportMgr)

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}
