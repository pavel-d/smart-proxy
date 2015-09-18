package traf_counter

import (
	"gopkg.in/redis.v3"
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
	t.Redis.IncrBy(TraffStatsRedisClientKey+IPAddrFromRemoteAddr(remote), bytesCount)
	t.Redis.IncrBy(TraffStatsRedisRemoteKey+host, bytesCount)
}

func IPAddrFromRemoteAddr(addr net.Addr) string {
	s := addr.String()
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}
