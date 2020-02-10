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

package product_pusher

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	pb "github.com/mattcary/microservices-demo/src/productcatalogservice/genproto"
)

const (
	CATALOG_FILE = "testdata/catalog.json"

	// The following should set up on a local DB, with admin privileges granted to a database "Catalog", eg
	// CREATE USER products@localhost IDENTIFIED BY "be84fa653f7770a";
	// CREATE DATABASE Catalog;
	// GRANT ALL PRIVILEGES ON Catalog.* TO products@localhost;
	USER = "products"
	PASSWORD = "be84fa653f7770a"
)

func openTestDB() (*sql.DB, error) {
	dbStr := fmt.Sprintf("%s:%s@/Catalog", USER, PASSWORD)
	return sql.Open("mysql", dbStr)
}

func connect(t *testing.T) *sql.DB {
	db, err := openTestDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS Products"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS Categories"); err != nil {
		t.Fatal(err)
	}

	return db
}

func checkTables(t *testing.T, db *sql.DB) {
	res, err := db.Query("show tables")
	if err != nil {
		t.Fatal(err)
	}
	tables := make(map[string]bool)
	for res.Next() {
		var table string
		if err = res.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables[table] = true
	}
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables instead of %v", len(tables))
	}
	for _, table := range []string{"Products", "Categories"} {
		if _, found := tables[table]; found == false {
			t.Errorf("Table %v not created", table)
		}
	}
}

func checkProducts(t *testing.T, db *sql.DB) {
	res, err := db.Query("SELECT ProductID from Products")
	ids := make(map[string]bool)
	for res.Next() {
		var id string
		if err = res.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[id] = true
	}
	if len(ids) != 2 {
		t.Errorf("Expected 2 products instead of %v", len(ids))
	}
	// These ids taken from testdata/catalog.json.
	for _, id := range []string{"OLJCESPC7Z", "66VCHSJNUP"} {
		if _, found := ids[id]; found == false {
			t.Errorf("Id %v not found", id)
		}
	}

	res, err = db.Query("SELECT Category FROM Categories WHERE ProductID = '66VCHSJNUP'")
	categories := make(map[string]bool)
	for res.Next() {
		var category string
		if err = res.Scan(&category); err != nil {
			t.Fatal(err)
		}
		categories[category] = true
	}
	if len(categories) != 2 {
		t.Errorf("Exepected 2 categories instead of %v", len(categories))
	}
	for _, c := range []string{"photography", "vintage"} {
		if _, found := categories[c]; found == false {
			t.Errorf("Category %v not found", c)
		}
	}
}

func TestMain(m *testing.M) {
	testStatus := m.Run()
	// Reconnect to clean up the DB.
	db, err := openTestDB()
	// TODO: figure out how to report an error here
	if err == nil {
		defer db.Close()
		_, _ = db.Exec("DROP TABLE Products");
		_, _ = db.Exec("DROP TABLE Categories");
	}
	os.Exit(testStatus)
}

func TestInit(t *testing.T) {
	db := connect(t)
	defer db.Close()

	// Check init from empty DB.
	err := initDB(db)
	if err != nil {
		t.Fatal(err)
	}
	checkTables(t, db)

	// Check init from DB already created.
	err = initDB(db)
	if err != nil {
		t.Fatal(err)
	}
	checkTables(t, db)
}

func TestWrite(t *testing.T) {
	db := connect(t)
	defer db.Close()

	err := initDB(db)
	if err != nil {
		t.Fatal(err)
	}

	var catalog *pb.ListProductsResponse
	catalog, err = ReadCatalogFile(CATALOG_FILE)
	if err != nil {
		t.Fatal(err)
	}
	err = writeCatalog(catalog, db)
	if err != nil {
		t.Fatal(err)
	}

	checkProducts(t, db)
}

func TestPush(t *testing.T) {
	os.Setenv("SQL_USER", "products")
	os.Setenv("SQL_PASSWORD", "be84fa653f7770a")
	os.Setenv("SQL_HOST", "")
	PushCatalog(CATALOG_FILE)

	db, err := openTestDB()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	checkProducts(t, db)
}
