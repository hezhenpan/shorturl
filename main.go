package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
	"github.com/thanhpk/randstr"
	"net/http"
	"os"
	"time"
)

type ucache interface {
	GenShortURL(string) (string, error)
	GetOriginUrl(string) (string, error)
}

type redisCache struct {
	rdb *redis.Client
	ctx context.Context
}

type memCache struct {
	ce *cache.Cache
}

var uk ucache

func main() {
	raddr := os.Getenv("REDISURI")
	if raddr == "" {
		uk = memCache{
			ce: cache.New(5*time.Minute, 10*time.Minute),
		}
	} else {
		uk = redisCache{
			rdb: redis.NewClient(&redis.Options{
				Addr:     raddr,
				Password: "", // no password set
				DB:       2,  // use DB 2
			}),
			ctx: context.Background(),
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/:ukey", GetUrlHandler)
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello, world!")
	})
	r.POST("/set/url", SetUrlHandler)
	r.Run(":8080")

}

func GetUrlHandler(c *gin.Context) {
	ukey := c.Param("ukey")
	url, err := uk.GetOriginUrl(ukey)
	if err != nil {
		c.String(http.StatusInternalServerError, "redirect url back error: %s", err.Error())
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func SetUrlHandler(c *gin.Context) {
	oriURL := c.PostForm("url")
	url, err := uk.GenShortURL(oriURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "e: %v", err.Error())
		return
	}
	c.String(http.StatusOK, "u short url is /%v", url)
}


func (m memCache) GenShortURL(url string) (string, error) {
	shortString := randstr.Base62(8)

	_, found := m.ce.Get(shortString)
	if found {
		return "", fmt.Errorf("please retry")
	}

	m.ce.Set(shortString, url, time.Hour*24)
	return shortString, nil

}

func (m memCache) GetOriginUrl(ukey string) (string, error) {
	rst, found := m.ce.Get(ukey)
	if found {
		return rst.(string), nil
	}
	return "", fmt.Errorf("no value for ukey %v", ukey)
}


// 1. gen random string for short url and redis key
// 2. use setnx to check if already exists, repeat
//

func (r redisCache) GenShortURL(url string) (string, error) {
	shortString := randstr.Base62(8)
	rst, err := r.rdb.SetNX(r.ctx, shortString, url, time.Hour*24).Result()
	if err != nil {
		return "", err
	}
	if rst {
		return shortString, nil
	}
	return "", fmt.Errorf("please retry")
}

func (r redisCache) GetOriginUrl(ukey string) (string, error) {
	rst, err := r.rdb.Get(r.ctx, ukey).Result()
	if err != nil {
		return "", err
	}
	return rst, nil
}


