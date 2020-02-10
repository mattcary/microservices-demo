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
	"os"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	pb "github.com/mattcary/microservices-demo/src/productcatalogservice/genproto"
	"github.com/mattcary/microservices-demo/src/util/product_pusher"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
	// This assumes a local MySQL instance set up as in the util/product_pusher test.
	os.Setenv("SQL_USER", "products")
	os.Setenv("SQL_PASSWORD", "be84fa653f7770a")
	os.Setenv("SQL_HOST", "")
	product_pusher.PushCatalog("products.json")
	rsp, err := product_pusher.ReadCatalogFile("products.json")
	if err != nil {
		t.Fatal(err)
	}
	parsedCatalog := rsp.Products

	err = openDB()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	addr := run("0")
	conn, err := grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewProductCatalogServiceClient(conn)
	res, err := client.ListProducts(ctx, &pb.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	expectedProducts := make(map[string]*pb.Product)
	for _, prod := range parsedCatalog {
		expectedProducts[prod.Id] = prod
	}

	for _, prod := range res.Products {
		expected, found := expectedProducts[prod.Id]
		if !found {
			t.Errorf("Unexpected product %v", prod.Id)
		} else if diff := cmp.Diff(prod, expected, cmp.Comparer(proto.Equal)); diff != "" {
			t.Error(diff)
		}
	}

	got, err := client.GetProduct(ctx, &pb.GetProductRequest{Id: "OLJCESPC7Z"})
	if err != nil {
		t.Fatal(err)
	}
	if want := expectedProducts["OLJCESPC7Z"]; !proto.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	_, err = client.GetProduct(ctx, &pb.GetProductRequest{Id: "N/A"})
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	sres, err := client.SearchProducts(ctx, &pb.SearchProductsRequest{Query: "typewriter"})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(sres.Results, []*pb.Product{expectedProducts["OLJCESPC7Z"]}, cmp.Comparer(proto.Equal)); diff != "" {
		t.Error(diff)
	}
}
