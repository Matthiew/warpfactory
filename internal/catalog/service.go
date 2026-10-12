package catalog

import (
	"context"
	"encoding/json"
	"net/http"

	catalogv1 "github.com/Matthiew/warpfactory/gen/catalog/v1"
)

type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
}

type Car struct {
	ID    string `json:"id"`
	Make  string `json:"make"`
	Model string `json:"model"`
}

var books = []Book{
	{ID: "book-1", Title: "The Hobbit", Author: "J. R. R. Tolkien"},
	{ID: "book-2", Title: "Dune", Author: "Frank Herbert"},
}

var cars = []Car{
	{ID: "car-1", Make: "Toyota", Model: "Corolla"},
	{ID: "car-2", Make: "Honda", Model: "Civic"},
}

type Service struct {
	catalogv1.UnimplementedCatalogServer
	config *Store
}

func NewService(config *Store) *Service {
	return &Service{config: config}
}

func (s *Service) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /books", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			CatalogLabel string `json:"catalog_label"`
			Books        []Book `json:"books"`
		}{CatalogLabel: s.config.Current().CatalogLabel, Books: books})
	})
	mux.HandleFunc("GET /cars", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			CatalogLabel string `json:"catalog_label"`
			Cars         []Car  `json:"cars"`
		}{CatalogLabel: s.config.Current().CatalogLabel, Cars: cars})
	})
	return mux
}

func (s *Service) ListBooks(context.Context, *catalogv1.ListBooksRequest) (*catalogv1.ListBooksResponse, error) {
	result := &catalogv1.ListBooksResponse{CatalogLabel: s.config.Current().CatalogLabel}
	for _, book := range books {
		result.Books = append(result.Books, &catalogv1.Book{Id: book.ID, Title: book.Title, Author: book.Author})
	}
	return result, nil
}

func (s *Service) ListCars(context.Context, *catalogv1.ListCarsRequest) (*catalogv1.ListCarsResponse, error) {
	result := &catalogv1.ListCarsResponse{CatalogLabel: s.config.Current().CatalogLabel}
	for _, car := range cars {
		result.Cars = append(result.Cars, &catalogv1.Car{Id: car.ID, Make: car.Make, Model: car.Model})
	}
	return result, nil
}
