package traf_counter

import (
	"gopkg.in/redis.v3"
	"log"
	"net"
	"strings"
)

const (
	TraffStatsRedisClientKey = "traff_stats:client:"
	TraffStatsRedisRemoteKey = "traff_stats:remote_host:"
)

type TrafCounter struct {
	Redis *redis.Client
}

func (t *TrafCounter) Count(host string, remote net.Addr, bytesCount int64) {
	log.Printf("Saving traf stats for %v -> %v, bytes count: %v", remote, host, bytesCount)
	_, err := t.Redis.IncrBy(TraffStatsRedisClientKey+IPAddrFromRemoteAddr(remote), bytesCount).Result()
	if err != nil {
		log.Printf("Failed to save trafstats for %v", TraffStatsRedisClientKey+IPAddrFromRemoteAddr(remote))
	} else {
		log.Printf("Increment traffic counter for %v: ,%v", TraffStatsRedisClientKey+IPAddrFromRemoteAddr(remote), bytesCount)
	}

	_, err = t.Redis.IncrBy(TraffStatsRedisRemoteKey+host, bytesCount).Result()
	if err != nil {
		log.Printf("Failed to save trafstats for %v", TraffStatsRedisRemoteKey+host)
	} else {
		log.Printf("Increment traffic counter for %v: ,%v", TraffStatsRedisRemoteKey+host, bytesCount)
	}
}

func IPAddrFromRemoteAddr(addr net.Addr) string {
	s := addr.String()
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}
