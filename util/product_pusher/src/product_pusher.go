// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	pb "github.com/mattcary/microservices-demo/src/productcatalogservice/genproto"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/protobuf/jsonpb"
	"github.com/sirupsen/logrus"
)

const (
	CATLOG_FILE  = "CATALOG_FILE"
	SQL_USER     = "SQL_USER"
	SQL_PASSWORD = "SQL_PASSWORD"
	SQL_HOST     = "SQL_HOST"
)

var (
	log *logrus.Logger
)

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

func readCatalogFile(filename string) (*pb.ListProductsResponse, error) {
	catalogJSON, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("failed to open product catalog json file: %v", err)
		return nil, err
	}
	var catalog pb.ListProductsResponse
	if err := jsonpb.Unmarshal(bytes.NewReader(catalogJSON), &catalog); err != nil {
		log.Warnf("failed to parse the catalog JSON: %v", err)
		return nil, err
	}
	log.Info("successfully parsed product catalog json")
	return &catalog, nil
}

func writeProduct(product *pb.Product, db *sql.DB) error {
	categoryIns, err := db.Prepare("INSERT INTO Categories VALUES (?, ?)")
	if err != nil {
		log.Warn("Could not prepare Category insert")
		return err
	}
	defer categoryIns.Close()

	_, err = db.Exec("INSERT INTO Products VALUES (?, ?, ?, ?, ?, ?, ?)", product.Id, product.Name, product.Description, product.Picture, product.PriceUsd.CurrencyCode, product.PriceUsd.Units, product.PriceUsd.Nanos)
	if err != nil {
		log.Warnf("Could not insert product %v", product)
		return err // TODO: err.wrap, structure errors rather than warning string.
	}
	for _, c := range product.Categories {
		_, err = categoryIns.Exec(product.Id, c)
		if err != nil {
			log.Warnf("Could not insert product %v category %v", product.Id, c)
			return err
		}
	}

	return nil
}

func writeCatalog(catalog *pb.ListProductsResponse, db *sql.DB) error {
	count := 0
	for _, product := range catalog.Products {
		err := writeProduct(product, db)
		if err != nil {
			log.Warnf("Could not write %v", product)
			return err
		}
		count++
	}
	log.Infof("Wrote %v products", count)
	return nil
}

func createTables(db *sql.DB) error {
	_, productErr := db.Exec(`
        CREATE TABLE Products (
            ProductID     varchar(32),
            Name          varchar(255),
            Description   varchar(1024),
            Picture       varchar(512),
            CurrencyCode  char(3),
            CurrencyUnits int(32),
            CurrencyNanos int(64))`)
	if productErr != nil {
		log.Warn("Could not create table Products")
		// Fall through to try to create Categories.
	}
	_, categoryErr := db.Exec(`
        CREATE TABLE Categories (
            ProductID     varchar(32),
            Category      varchar(255))`)
	if categoryErr != nil {
		log.Warn("Could not create table Categories")
		// Fall through to return first err.
	}

	if productErr != nil {
		return productErr
	}
	if categoryErr != nil {
		return categoryErr
	}
	log.Info("Successfully created Products and Categories tables")
	return nil
}

func openDB() (*sql.DB, error) {
	dbStr := fmt.Sprintf("%s:%s@%s/Catalog", os.Getenv(SQL_USER), os.Getenv(SQL_PASSWORD), os.Getenv(SQL_HOST))
	db, err := sql.Open("mysql", dbStr)
	if err != nil {
		log.Warnf("Could not open %v", dbStr)
		return nil, err
	}
	return db, nil
}

func initDB(db *sql.DB) error {
	res, err := db.Query(`
        SELECT COUNT(*) FROM information_schema.tables
        WHERE table_schema = 'Catalog' AND
              (table_name = 'Products' OR table_name = 'Categories')`)
	if err != nil {
		log.Warnf("Could not check tables")
		return err
	}
	defer res.Close()

	tablesOkay := false
	if res.Next() {
		var count int
		if err := res.Scan(&count); err != nil {
			log.Warn("Could not read table count")
			return err
		}
		if count == 2 {
			tablesOkay = true
		}
	}
	if !tablesOkay {
		if err := createTables(db); err != nil {
			return err
		}
	}
	return nil
}
