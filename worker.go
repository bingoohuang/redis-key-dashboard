package rediskeydashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis"
)

func Scanner() {
	for {
		if ScanStatus == StatusWorker {
			ScanStatus = StatusProcess
			RedisInfo.StartTime = time.Now()

			scan()

			RedisInfo.EndTime = time.Now()
			ScanStatus = StatusReady
		}

		time.Sleep(time.Duration(1) * time.Second)
	}
}

func scan() {
	client := redis.NewClient(&redis.Options{
		Addr:        ScanConfReq.ServerAddress,
		Password:    ScanConfReq.Password,
		DB:          0,
		ReadTimeout: 1 * time.Minute,
	})
	defer client.Close()

	redisI, _ := client.Do("MEMORY", "STATS").Result()
	if stats, ok := redisI.([]interface{}); ok {
		for i := 0; i < len(stats); i += 2 {
			switch stats[i] {
			case "total.allocated":
				if i+1 < len(stats) {
					RedisInfo.TotalMemory = stats[i+1].(int64)
				}
			case "keys.count":
				if i+1 < len(stats) {
					RedisInfo.TotalKeyCount = stats[i+1].(int64)
				}

			}
		}
	}

	mr := KeyReports{}
	delimiters := strings.Split(ScanConfReq.Delimiters, ",")
	cursor := uint64(0)
	groupKey := ""

	isGroupKey := ScanConfReq.GroupKey
	isMemoryUsage := ScanConfReq.MemoryUsage

	for {
		keys, cursor, err := client.Scan(cursor, ScanConfReq.Pattern, 1000).Result()
		if err != nil {
			ScanStatus = StatusFail
			ScanErrMsg = "Redis not connect !! => " + ScanConfReq.ServerAddress
			break
		}

		for _, key := range keys {
			scanKey(client, isGroupKey, isMemoryUsage, key, delimiters, groupKey, mr)
		}

		if cursor == 0 {
			break
		}
	}

	if isMemoryUsage {
		for _, report := range mr {
			SortedReportListBySize = append(SortedReportListBySize, report)
		}
		sort.Sort(SortedReportListBySize)
	} else {
		for _, report := range mr {
			SortedReportListByCount = append(SortedReportListByCount, report)
		}
		sort.Sort(SortedReportListByCount)
	}
}

func scanKey(client *redis.Client, isGroupKey, isMemoryUsage bool, key string, delimiters []string, groupKey string, mr KeyReports) {
	var memoryUsage int64
	if isMemoryUsage {
		memoryUsage, _ = client.MemoryUsage(key).Result()
	}

	if !isGroupKey {
		mr[key] = Report{Key: key, Count: 1, Size: memoryUsage}
		return
	}

	if len(delimiters) <= 1 {
		groupKey = key
	} else {
		for _, delimiter := range delimiters {
			tmp := strings.Split(key, delimiter)
			if len(tmp) > 1 {
				groupKey = strings.Join(tmp[0:len(tmp)-1], delimiter) + delimiter + "*"
				break
			}

			groupKey = key
		}
	}

	r := Report{}
	if _, ok := mr[groupKey]; ok {
		r = mr[groupKey]
	} else {
		r = Report{Key: groupKey}
	}

	r.Size += memoryUsage
	r.Count++
	mr[groupKey] = r
}
