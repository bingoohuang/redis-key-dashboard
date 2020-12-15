package rediskeydashboard

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"math"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/bingoohuang/pkger"

	"github.com/averagesecurityguy/random"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/skratchdot/open-golang/open"
)

type ContextPath struct {
	ContextPath string
	AuthFn      gin.HandlerFunc
	IndexTpl    *template.Template
}

func MakeContextPath(p string, auth string) ContextPath {
	authFn := BasicAuthCheck(auth)
	pkger.Include("/assets")

	indexHtml, err := pkger.Read("/assets/index.html")
	if err != nil {
		panic(err)
	}
	indexTpl, err := template.New("index.html").Funcs(fnMap).Parse(string(indexHtml))
	if err != nil {
		panic(err)
	}

	return ContextPath{ContextPath: fixContextPath(p), AuthFn: authFn, IndexTpl: indexTpl}
}

func fixContextPath(p string) string {
	if p == "/" || p == "" {
		return ""
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	if strings.HasSuffix(p, "/") {
		p = p[0 : len(p)-1]
	}

	return p
}

// OpenExplorer ...
func (cp ContextPath) OpenExplorer(port int) {
	switch runtime.GOOS {
	case "windows", "darwin":
		n, _ := random.AlphaNum(10)
		addr := fmt.Sprintf("http://127.0.0.1:%d%s?%s", port, cp.ContextPath, n)
		_ = open.Run(addr)
	}
}

func (cp ContextPath) Path(p string) string {
	return path.Join(cp.ContextPath, p)
}

func (cp ContextPath) AssetsHandler(c *gin.Context) {
	name := c.Param("name")
	raw, err := pkger.Read("/assets/" + name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Data(http.StatusOK, mime.TypeByExtension(filepath.Ext(name)), raw)
}

func (cp ContextPath) MainHandler(c *gin.Context) {
	var workerTime float64
	if RedisInfo.EndTime.IsZero() {
		workerTime = time.Now().Sub(RedisInfo.StartTime).Seconds()
	} else {
		workerTime = RedisInfo.EndTime.Sub(RedisInfo.StartTime).Seconds()
	}

	report1Len, report2Len := 0, 0
	if len(SortedReportListByCount) < 25 {
		report1Len = len(SortedReportListByCount)
	} else {
		report1Len = 25
	}

	if len(SortedReportListBySize) < 25 {
		report2Len = len(SortedReportListBySize)
	} else {
		report2Len = 25
	}

	err := cp.IndexTpl.Execute(c.Writer, map[string]interface{}{
		"contextPath":             cp.ContextPath,
		"status":                  ScanStatus,
		"scanErrMsg":              ScanErrMsg,
		"scanConfReq":             ScanConfReq,
		"redisInfo":               RedisInfo,
		"workerTime":              workerTime,
		"sortedReportListByCount": SortedReportListByCount[0:report1Len],
		"sortedReportListBySize":  SortedReportListBySize[0:report2Len],
	})
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

func (cp ContextPath) ResetWorkerHandler(*gin.Context) {
	ScanStatus = StatusIdle
	ScanErrMsg = ""
	RedisInfo = RedisInfoStruct{}
	ScanConfReq = ScanConfReqStruct{}
	SortedReportListByCount = SortByCount{}
	SortedReportListBySize = SortBySize{}
}

func (cp ContextPath) WorkerHandler(c *gin.Context) {
	if err := c.ShouldBindWith(&ScanConfReq, binding.Form); err != nil {
		c.JSON(401, gin.H{"message": "Invalid Form", "response": "err"})
		return
	}

	ScanStatus = StatusWorker
	c.JSON(200, gin.H{"response": "success"})
}

func (cp ContextPath) CheckStatusHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": ScanStatus})
}

func (cp ContextPath) CsvExportHandler(c *gin.Context) {
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=report:"+ScanConfReq.ServerAddress+".csv")

	b := &bytes.Buffer{}
	w := csv.NewWriter(b)

	if err := w.Write([]string{"Key", "Count", "Size"}); err != nil {
		log.Fatalln("error writing record to csv:", err)
	}

	isMemoryUsage := ScanConfReq.MemoryUsage
	if !isMemoryUsage {
		for _, d := range SortedReportListByCount {
			w.Write([]string{d.Key, strconv.FormatInt(d.Count, 10), "-"})
		}
	} else {
		for _, d := range SortedReportListBySize {
			w.Write([]string{d.Key, strconv.FormatInt(d.Count, 10), strconv.FormatInt(d.Size, 10)})
		}
	}
	w.Flush()

	c.Data(http.StatusOK, "text/csv", b.Bytes())
}

func BasicAuthCheck(auth string) gin.HandlerFunc {
	if auth == "" {
		return nil
	}

	authHead := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	return func(c *gin.Context) {
		if authHead == c.GetHeader("Authorization") {
			return
		}

		c.Header("WWW-Authenticate", "Authorization Required")
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}

var fnMap = template.FuncMap{
	"indexView": func(s int) string {
		return fmt.Sprintf("%d", s+1)
	},
	"formatMib": func(s int64) string {
		return fmt.Sprintf("%.5f MiB", float64(s)/1024/1024)
	},
	"formatMibRaw": func(s int64) float64 {
		return math.Round(float64(s)/1024/1024*10000) / 10000
	},
}
