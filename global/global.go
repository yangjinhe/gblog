package global

import (
	"encoding/json"
	"fmt"
	"github.com/blevesearch/bleve"
	gcore "github.com/snail007/gmc/core"
	gcast "github.com/snail007/gmc/util/cast"
	gmap "github.com/snail007/gmc/util/map"
	"math/rand"
	"strings"
)

var (
	Context *BContext
)

type BContext struct {
	db         gcore.Database
	cache      gcore.Cache
	log        gcore.Logger
	config     gcore.Config
	configFile string
	isDebug    bool
	server     gcore.HTTPServer
	indexer    bleve.Index
}

func (B *BContext) Indexer() bleve.Index {
	return B.indexer
}

func (B *BContext) SetIndexer(indexer bleve.Index) {
	B.indexer = indexer
}

func (B *BContext) Server() gcore.HTTPServer {
	return B.server
}

func (B *BContext) SetServer(server gcore.HTTPServer) {
	B.server = server
}

func (B *BContext) ConfigFile() string {
	return B.configFile
}

func (B *BContext) Config() gcore.Config {
	return B.config
}

// key can be:
// "basic.web_site_title": string,
// "basic": map[string]interface{}
func (B *BContext) BConfig(key string,defaultValue ...string) (value interface{}) {
	defaultValue0:=""
	if len(defaultValue)>0{
		defaultValue0=defaultValue[0]
	}
	db := B.DB()
	keyArr := strings.Split(key, ".")
	l := len(keyArr)
	configType := keyArr[0]
	rs, err := db.Query(db.AR().Cache("allConfig", 3600).From("config"))
	if err != nil {
		return defaultValue0
	}
	allConfig := rs.MapRows("key")
	if v, ok := allConfig[configType]; !ok {
		return defaultValue0
	} else {
		value := gmap.M{}
		err = json.Unmarshal([]byte(v["value"]), &value)
		if err != nil {
			return defaultValue0
		}
		if l == 1 {
			return value
		}
		if v, ok := value[keyArr[1]]; !ok {
			return defaultValue0
		} else {
			return gcast.ToString(v)
		}
	}
	return defaultValue0
}

func (B *BContext) Log() gcore.Logger {
	return B.log
}

func (B *BContext) SetLog(log gcore.Logger) {
	B.log = log
}

func (B *BContext) Cache() gcore.Cache {
	return B.cache
}

func (B *BContext) SetCache(cache gcore.Cache) {
	B.cache = cache
}

func (B *BContext) DB() gcore.Database {
	return B.db
}

func (B *BContext) SetDB(db gcore.Database) {
	B.db = db
}
func (B *BContext) IsDebug() bool {
	return B.isDebug
}

func (B *BContext) SetIsDebug(isDebug bool) {
	B.isDebug = isDebug
}

func NewBContext(config gcore.Config) (c *BContext, err error) {
	c = &BContext{
		configFile: config.ConfigFileUsed(),
		config:     config,
	}
	return
}

var (
	maxImgIdx int32 = 20
)

func RandImgIdx() string {
	return fmt.Sprintf("/static/style/%.3d.png", rand.Int31n(maxImgIdx)+1)
}
