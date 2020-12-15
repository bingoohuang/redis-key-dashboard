package main

import (
	"flag"
	"fmt"

	"github.com/bingoohuang/rediskeydashboard"
	"github.com/gin-gonic/gin"
)

func main() {
	contextPath := flag.String("contextPath", "", "contextPath")
	basicAuth := flag.String("auth", "", "Basic Auth for Admin User, eg admin:admin")
	serverPort := flag.Int("port", 8080, "Server Port")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	cp := rediskeydashboard.MakeContextPath(*contextPath, *basicAuth)

	r.GET(cp.Path("/"), cp.MainHandler)
	r.GET(cp.Path("/assets/*name"), cp.AssetsHandler)
	r.POST(cp.Path("/api/worker"), cp.WorkerHandler)
	r.POST(cp.Path("/api/reset-worker"), cp.ResetWorkerHandler)
	r.POST(cp.Path("/api/check-status"), cp.CheckStatusHandler)
	r.GET(cp.Path("/api/csv-export"), cp.CsvExportHandler)

	rediskeydashboard.ScanStatus = rediskeydashboard.StatusIdle
	go rediskeydashboard.Scanner()

	r.Run(fmt.Sprintf(":%d", *serverPort))
}
