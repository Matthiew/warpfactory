package catalog

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	catalogv1 "github.com/Matthiew/warpfactory/gen/catalog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestHTTPAndGRPCLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, `{"catalog_label":"initial"}`)
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)

	httpServer := httptest.NewServer(service.HTTPHandler())
	defer httpServer.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	catalogv1.RegisterCatalogServer(grpcServer, service)
	go grpcServer.Serve(listener)
	defer grpcServer.Stop()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := catalogv1.NewCatalogClient(conn)

	for _, label := range []string{"initial", "updated"} {
		if label == "updated" {
			writeConfig(t, path, `{"catalog_label":"updated"}`)
			if err := store.Reload(); err != nil {
				t.Fatal(err)
			}
		}
		assertHTTPList(t, httpServer.URL+"/books", label, "books", "The Hobbit")
		assertHTTPList(t, httpServer.URL+"/cars", label, "cars", "Corolla")

		booksResponse, err := client.ListBooks(context.Background(), &catalogv1.ListBooksRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if booksResponse.GetCatalogLabel() != label || len(booksResponse.GetBooks()) != 2 || booksResponse.GetBooks()[0].GetTitle() != "The Hobbit" {
			t.Fatalf("ListBooks: %+v", booksResponse)
		}
		carsResponse, err := client.ListCars(context.Background(), &catalogv1.ListCarsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if carsResponse.GetCatalogLabel() != label || len(carsResponse.GetCars()) != 2 || carsResponse.GetCars()[0].GetModel() != "Corolla" {
			t.Fatalf("ListCars: %+v", carsResponse)
		}
	}

	response, err := http.Get(httpServer.URL + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path: status %d", response.StatusCode)
	}
}

func assertHTTPList(t *testing.T, url, label, key, firstValue string) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%s: status %d, content type %q", url, response.StatusCode, response.Header.Get("Content-Type"))
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	var actualLabel string
	if err := json.Unmarshal(body["catalog_label"], &actualLabel); err != nil {
		t.Fatal(err)
	}
	if actualLabel != label {
		t.Fatalf("%s: label %q, want %q", url, actualLabel, label)
	}
	var items []map[string]string
	if err := json.Unmarshal(body[key], &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("%s: items %+v", url, items)
	}
	if items[0]["title"] != firstValue && items[0]["model"] != firstValue {
		t.Fatalf("%s: first item %+v", url, items[0])
	}
}

func TestInvalidReloadKeepsLastValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, `{"catalog_label":"good"}`)
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		`{`,
		`{"catalog_label":""}`,
		`{"catalog_label":"   "}`,
		`{"catalog_label":"new","unexpected":true}`,
		`{"catalog_label":"new"} {"catalog_label":"other"}`,
	} {
		writeConfig(t, path, invalid)
		if err := store.Reload(); err == nil {
			t.Fatalf("accepted invalid config %q", invalid)
		}
		if got := store.Current().CatalogLabel; got != "good" {
			t.Fatalf("invalid reload changed label to %q", got)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(); err == nil {
		t.Fatal("accepted missing config")
	}
	if got := store.Current().CatalogLabel; got != "good" {
		t.Fatalf("missing config changed label to %q", got)
	}
}

func TestConcurrentReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeConfig(t, path, `{"catalog_label":"one"}`)
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	failures := make(chan string, 8)
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 10000 {
				label := store.Current().CatalogLabel
				if label != "one" && label != "two" {
					failures <- label
					return
				}
			}
		}()
	}
	for range 20 {
		writeConfig(t, path, `{"catalog_label":"two"}`)
		if err := store.Reload(); err != nil {
			t.Fatal(err)
		}
		writeConfig(t, path, `{"catalog_label":"one"}`)
		if err := store.Reload(); err != nil {
			t.Fatal(err)
		}
	}
	readers.Wait()
	close(failures)
	for label := range failures {
		t.Errorf("observed invalid snapshot %q", label)
	}
}

func writeConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
