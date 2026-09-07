package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"time"
)

func cacheSetToken(token Token) error {
	key := common.GenerateHMAC(token.Key)
	token.Clean()
	return common.RedisHSetObj(fmt.Sprintf("token:%s", key), &token, time.Duration(common.RedisKeyCacheSeconds())*time.Second)
}
func cacheDeleteToken(key string) error {
	return common.RedisDelKey(fmt.Sprintf("token:%s", common.GenerateHMAC(key)))
}
func cacheIncrTokenQuota(key string, increment int64) error {
	return common.RedisHIncrBy(fmt.Sprintf("token:%s", common.GenerateHMAC(key)), constant.TokenFiledRemainQuota, increment)
}
func cacheDecrTokenQuota(key string, decrement int64) error {
	return cacheIncrTokenQuota(key, -decrement)
}
func cacheSetTokenField(key, field, value string) error {
	return common.RedisHSetField(fmt.Sprintf("token:%s", common.GenerateHMAC(key)), field, value)
}
func getTokenCacheKey(key string) string { return fmt.Sprintf("token:%s", common.GenerateHMAC(key)) }
func cacheGetTokenByKey(key string) (*Token, error) {
	var token Token
	if err := common.RedisHGetObj(getTokenCacheKey(key), &token); err != nil {
		return nil, err
	}
	token.Key = key
	return &token, nil
}
