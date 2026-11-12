package main

import (
	conf "github.com/wtwinlab/Altun/config"
	dbm "github.com/wtwinlab/Altun/layers/storage/db"
)

type DBContext struct {
	ID     string
	Config *conf.Config
}

type DBProvider func(*DBContext) (dbm.DB, error)

func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.BackendType(ctx.Config.DBBackend)

	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir())
}
