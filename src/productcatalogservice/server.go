// Copyright 2018 Google LLC
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
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	pb "github.com/mattcary/microservices-demo/src/productcatalogservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"cloud.google.com/go/profiler"
	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/stackdriver"
	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log          *logrus.Logger
	extraLatency time.Duration
	db           *sql.DB

	port = "3550"
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

func main() {
	go initTracing()
	go initProfiling("productcatalogservice", "1.0.0")
	flag.Parse()

	// set injected latency
	if s := os.Getenv("EXTRA_LATENCY"); s != "" {
		v, err := time.ParseDuration(s)
		if err != nil {
			log.Fatalf("failed to parse EXTRA_LATENCY (%s) as time.Duration: %+v", v, err)
		}
		extraLatency = v
		log.Infof("extra latency enabled (duration: %v)", extraLatency)
	} else {
		extraLatency = time.Duration(0)
	}

	if err := openDB(); err != nil {
		log.Warnf("Could not open database: %v", err)
	}
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	log.Infof("starting grpc server at :%s", port)
	run(port)
	select {}
}

func openDB() error {
	// open db
	dbStr := fmt.Sprintf("%s:%s@%s/Catalog", os.Getenv("SQL_USER"), os.Getenv("SQL_PASSWORD"), os.Getenv("SQL_HOST"))
	var err error
	db, err = sql.Open("mysql", dbStr)
	return err
}

func run(port string) string {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	svc := &productCatalog{}
	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	go srv.Serve(l)
	return l.Addr().String()
}

func initJaegerTracing() {
	svcAddr := os.Getenv("JAEGER_SERVICE_ADDR")
	if svcAddr == "" {
		log.Info("jaeger initialization disabled.")
		return
	}
	// Register the Jaeger exporter to be able to retrieve
	// the collected spans.
	exporter, err := jaeger.NewExporter(jaeger.Options{
		Endpoint: fmt.Sprintf("http://%s", svcAddr),
		Process: jaeger.Process{
			ServiceName: "productcatalogservice",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	log.Info("jaeger initialization completed.")
}

func initStats(exporter *stackdriver.Exporter) {
	view.SetReportingPeriod(60 * time.Second)
	view.RegisterExporter(exporter)
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Info("Error registering default server views")
	} else {
		log.Info("Registered default server views")
	}
}

func initStackdriverTracing() {
	// TODO(ahmetb) this method is duplicated in other microservices using Go
	// since they are not sharing packages.
	for i := 1; i <= 3; i++ {
		exporter, err := stackdriver.NewExporter(stackdriver.Options{})
		if err != nil {
			log.Warnf("failed to initialize Stackdriver exporter: %+v", err)
		} else {
			trace.RegisterExporter(exporter)
			trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
			log.Info("registered Stackdriver tracing")

			// Register the views to collect server stats.
			initStats(exporter)
			return
		}
		d := time.Second * 10 * time.Duration(i)
		log.Infof("sleeping %v to retry initializing Stackdriver exporter", d)
		time.Sleep(d)
	}
	log.Warn("could not initialize Stackdriver exporter after retrying, giving up")
}

func initTracing() {
	initJaegerTracing()
	initStackdriverTracing()
}

func initProfiling(service, version string) {
	// TODO(ahmetb) this method is duplicated in other microservices using Go
	// since they are not sharing packages.
	for i := 1; i <= 3; i++ {
		if err := profiler.Start(profiler.Config{
			Service:        service,
			ServiceVersion: version,
			// ProjectID must be set if not running on GCP.
			// ProjectID: "my-project",
		}); err != nil {
			log.Warnf("failed to start profiler: %+v", err)
		} else {
			log.Info("started Stackdriver profiler")
			return
		}
		d := time.Second * 10 * time.Duration(i)
		log.Infof("sleeping %v to retry initializing Stackdriver profiler", d)
		time.Sleep(d)
	}
	log.Warn("could not initialize Stackdriver profiler after retrying, giving up")
}

func readProductResult(rows *sql.Rows) (*pb.Product, error) {
	var (
		id            string
		name          string
		description   string
		picture       string
		currencyCode  string
		currencyUnits int64
		currencyNanos int32
	)
	if err := rows.Scan(&id, &name, &description, &picture, &currencyCode, &currencyUnits, &currencyNanos); err != nil {
		return nil, err
	}
	var p pb.Product
	p.Id = id
	p.Name = name
	p.Description = description
	p.Picture = picture
	var price pb.Money
	price.CurrencyCode = currencyCode
	price.Units = currencyUnits
	price.Nanos = currencyNanos
	p.PriceUsd = &price
	return &p, nil
}

func getProductCategories(db *sql.DB, id string) (*[]string, error) {
	res, err := db.Query("SELECT Category FROM Categories WHERE ProductID = ?", id)
	if err != nil {
		return nil, err
	}
	var categories []string
	for res.Next() {
		var category string
		err = res.Scan(&category)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return &categories, nil
}

type productCatalog struct{}

func (p *productCatalog) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (p *productCatalog) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (p *productCatalog) ListProducts(context.Context, *pb.Empty) (*pb.ListProductsResponse, error) {
	time.Sleep(extraLatency)
	res, err := db.Query("SELECT ProductID, Name, Description, Picture, CurrencyCode, CurrencyUnits, CurrencyNanos FROM Products")
	if err != nil {
		return nil, err
	}
	var products []*pb.Product
	for res.Next() {
		prod, err := readProductResult(res)
		if err != nil {
			return nil, err
		}
		products = append(products, prod)
	}

	categories := make(map[string][]string)
	res, err = db.Query("SELECT ProductID, Category FROM Categories")
	for res.Next() {
		var id string
		var category string
		err = res.Scan(&id, &category)
		if err != nil {
			return nil, err
		}
		categories[id] = append(categories[id], category)
	}
	for _, prod := range products {
		cs, found := categories[prod.Id]
		if found {
			for _, c := range cs {
				prod.Categories = append(prod.Categories, c)
			}
		}
	}
	return &pb.ListProductsResponse{Products: products}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	time.Sleep(extraLatency)
	res, err := db.Query("SELECT ProductID, Name, Description, Picture, CurrencyCode, CurrencyUnits, CurrencyNanos FROM Products WHERE ProductID = ?", req.Id)
	if err != nil {
		return nil, err
	}
	if !res.Next() {
		return nil, status.Errorf(codes.NotFound, "No matching products for %v", req.Id)
	}
	prod, err := readProductResult(res)
	categories, err := getProductCategories(db, prod.Id)
	if err != nil {
		return nil, err
	}
	if categories != nil {
		prod.Categories = *categories
	}
	return prod, nil
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	time.Sleep(extraLatency)
	// Intepret query as a substring match in name or description.
	res, err := db.Query("SELECT ProductID, Name, Description, Picture, CurrencyCode, CurrencyUnits, CurrencyNanos FROM Products WHERE Name LIKE CONCAT('%', ?, '%') OR Description LIKE CONCAT('%', ?, '%')", req.Query, req.Query)
	if err != nil {
		return nil, err
	}
	var products []*pb.Product
	for res.Next() {
		prod, err := readProductResult(res)
		if err != nil {
			return nil, err
		}
		products = append(products, prod)
	}
	for _, p := range products {
		categories, err := getProductCategories(db, p.Id)
		if err != nil {
			return nil, err
		}
		if categories != nil {
			p.Categories = *categories
		}
	}
	return &pb.SearchProductsResponse{Results: products}, nil
}
