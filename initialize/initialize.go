// Copyright 2020 The GMC Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// More information at https://github.com/snail007/gmc

package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gblog/global"
	"gblog/router"
	"github.com/snail007/gmc"
	gcore "github.com/snail007/gmc/core"
	"github.com/snail007/gmc/http/server"
	gcache "github.com/snail007/gmc/module/cache"
	gdb "github.com/snail007/gmc/module/db"
	glog "github.com/snail007/gmc/module/log"
	gfile "github.com/snail007/gmc/util/file"
)

func Initialize(s *ghttpserver.HTTPServer) (err error) {
	defer func() {
		if err != nil {
			err = gmc.Err.Wrap(err)
		}
	}()
	isDebugI, _ := s.Ctx().Get("debug")
	isDebug := isDebugI.(bool)

	if isDebug {
		s.Logger().Infof("gblog running in debug mode")
		s.Config().Debug()
	}

	dir, _ := os.Getwd()
	s.Logger().Infof("working dir : %s", gfile.Abs(dir))
	s.Logger().Infof("config file used: %s", gfile.Abs(s.Config().ConfigFileUsed()))

	// init Context
	ctx, err := global.NewBContext(s.Config())
	if err != nil {
		return
	}
	ctx.SetIsDebug(isDebug)
	ctx.SetServer(gcore.HTTPServer(s))
	cfg := ctx.Config()

	//init logger
	ctx.SetLog(s.Logger())

	// init db directory
	dataFile := cfg.Get("database.sqlite3").([]interface{})[0].(map[string]interface{})["database"].(string)
	dataDir := filepath.Dir(dataFile)

	s.Logger().Infof("data dir used: %s", gfile.Abs(dataDir))
	if !gfile.Exists(dataDir) {
		err = os.MkdirAll(dataDir, 0700)
		if err != nil {
			return
		}
	}

	// init db
	err = gdb.Init(cfg)
	if err != nil {
		return
	}
	ctx.SetDB(gdb.DB())
	if _, ok := gdb.DB().(*gdb.MySQLDB); ok {
		gdb.DBMySQL().Config.Cache = &DBCache{}
	} else {
		gdb.DBSQLite3().Config.Cache = &DBCache{}
	}
	// auto init databases
	isNewDB, err := checkTable(ctx.DB())
	if err != nil {
		return
	}
	// auto update db tables, if needed.
	if !isNewDB {
		err = autoUpdateTable(ctx.DB())
		if err != nil {
			return
		}
	}

	// init logger
	logger := glog.NewFromConfig(cfg, "")
	if ctx.IsDebug() {
		logger.SetLevel(gcore.LogLevelTrace)
	}
	ctx.SetLog(logger)

	//init cache
	err = gcache.Init(cfg)
	if err != nil {
		return
	}
	ctx.SetCache(gcache.Cache())

	// init indexer
	if cfg.GetBool("search.enablefulltextindex") {
		err = initIndexer(ctx)
		if err != nil {
			return
		}
	}

	//register ctx to global
	global.Context = ctx

	// init router
	router.InitRouter(s)
	return
}

type DBCache struct{}

func (s *DBCache) Set(key string, val []byte, expire uint) (err error) {
	return global.Context.Cache().Set(key, string(val), time.Second*time.Duration(expire))
}

func (s *DBCache) Get(key string) (data []byte, err error) {
	d, err := global.Context.Cache().Get(key)
	if err != nil {
		return
	}
	data = []byte(d)
	return
}

func checkTable(db gcore.Database) (isNewDB bool, err error) {
	isNewDB = true
	_, e := db.Query(db.AR().Raw(prefixCheck(db, "select * from __PREFIX__article")))
	if e == nil {
		isNewDB = false
		return
	}
	now := time.Now().Unix()
	sql := `
create table __PREFIX__article(
  article_id integer  PRIMARY KEY AUTOINCREMENT,
  title text,
  summary text,
  poster_url text,
  content text,
  is_draft int default 0,
  catalog_id int,
  create_time int,
  update_time int
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4_general_ci;
create table __PREFIX__catalog(
  catalog_id integer PRIMARY KEY AUTOINCREMENT,
  name text,
  sequence int default 0,
  is_nav int default 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4_general_ci;
CREATE TABLE __PREFIX__user (
  user_id integer PRIMARY KEY AUTOINCREMENT,
  username text,
  nickname text,
  password text,
  is_delete int default 0,
  update_time int,
  create_time int
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4_general_ci;
create table __PREFIX__config(
  config_id integer PRIMARY KEY AUTOINCREMENT,
  key text,
  value text
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4_general_ci;
insert into __PREFIX__config (config_id, key, value) values (1,"basic",'{"file":"","key":"basic","web_site_blogger_email":"gblog@example.com","web_site_blogger_name":"又一个gblog博客","web_site_blogger_site":"https://github.com/snail007","web_site_copyright":"本博客内容，gblog版权所有","web_site_description":"gblog是一个广受欢迎的个人开源博客系统，使用golang开发，使用简单，专业的个人博客系统。","web_site_icp":"","web_site_keywords":"gblog开源博客，gmc框架，go博客系统，开源博客","web_site_logo":"/static/style/logo.png","web_site_icon":"/static/style/favicon.ico","web_site_stat":"","web_site_status":"on","web_site_title":"又一个gblog博客！"}');
insert into __PREFIX__config (config_id, key, value) values (2,"system","{}");
insert into __PREFIX__config (config_id, key, value) values (3,"upload",'{"github_repo":"","github_speed_url":"https://cdn.jsdelivr.net/gh/%u/%p","github_token":"","gitlab_api_url":"","gitlab_repo":"","gitlab_token":"","image_mask_text":"","image_resize_width":"1024","key":"upload","upload_file_storage":"local","upload_image_compress":"5"}');
insert into __PREFIX__catalog (catalog_id, name, sequence) values (0,"默认分类",0);
insert into __PREFIX__user (user_id, username, nickname, password, is_delete, update_time, create_time) values (1,'root',	'root',	'2df594b9710111099edbdb7edaa43301',	0,	{now},	{now});
`
	sql = prefixCheck(db, sql)
	if _, ok := db.(*gdb.MySQLDB); ok {
		sql = strings.Replace(sql, "integer", "int", -1)
		sql = strings.Replace(sql, "AUTOINCREMENT", "AUTO_INCREMENT", -1)
		sql = strings.Replace(sql, "key text", "`key` text", 1)
		sql = strings.Replace(sql, "key, value", "`key`, value ", -1)
		sql = strings.Replace(sql, "key, value", "`key`, value ", -1)
	} else {
		sql = strings.Replace(sql, "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4_general_ci", "", -1)
	}
	sql = strings.Replace(sql, "{now}", fmt.Sprintf("%d", now), -1)
	// create table
	for _, v := range strings.Split(strings.Trim(sql, ";\n\t "), ";") {
		if v != "" {
			_, err = db.Exec(db.AR().Raw(v))
			if err != nil {
				return
			}
		}
	}
	return
}

func prefixCheck(db gcore.Database, sql string) string {
	if v, ok := db.(*gdb.MySQLDB); ok {
		if v.Config.TablePrefix == "" {
			sql = strings.Replace(sql, "__PREFIX__", "", -1)
		}
	} else if v, ok := db.(*gdb.SQLite3DB); ok {
		if v.Config.TablePrefix == "" {
			sql = strings.Replace(sql, "__PREFIX__", "", -1)
		}
	}
	return sql
}

func autoUpdateTable(db gcore.Database) (err error) {
	// checking sql, checking result except not nil, update sql.
	sqlData := [][]interface{}{
		//{"select is_draft from __PREFIX__article limit 0,1", true, `alter table __PREFIX__article add column is_draft integer default 0`},
	}
	for _, v := range sqlData {
		checkSQL := v[0].(string)
		checkSQLExceptNotNil := v[1].(bool)
		updateSQL := v[2].(string)
		// add article.is_draft
		_, e := db.Query(db.AR().Raw(checkSQL))
		if (checkSQLExceptNotNil && e != nil) || (!checkSQLExceptNotNil && e == nil) {
			for _, sql := range strings.Split(strings.Trim(updateSQL, ";\n\t "), ";") {
				if sql != "" {
					_, err = db.Exec(db.AR().Raw(sql))
					if err != nil {
						return
					}
				}
			}
		}
	}
	return
}
