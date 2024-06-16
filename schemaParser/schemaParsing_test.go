package schemaParser

import (
	"encoding/json"
	"fmt"
	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fileutil "graphql-go-schema-parser/util"
	"log"
	"os"
	"testing"
)

type Book struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PageCount int    `json:"pageCount"`
	AuthorID  string `json:"authorId"`
}

var books = []Book{
	{ID: "book-1", Name: "Harry Potter and the Philosopher's Stone", PageCount: 223, AuthorID: "author-1"},
	{ID: "book-2", Name: "Moby Dick", PageCount: 635, AuthorID: "author-2"},
	{ID: "book-3", Name: "Interview with the Vampire", PageCount: 371, AuthorID: "author-3"},
}

func getById(id string) *Book {
	for _, book := range books {
		if book.ID == id {
			return &book
		}
	}
	return nil
}

type Author struct {
	ID        string
	FirstName string
	LastName  string
}

var authors = []Author{
	{ID: "author-1", FirstName: "Joanne", LastName: "Rowling"},
	{ID: "author-2", FirstName: "Herman", LastName: "Melville"},
	{ID: "author-3", FirstName: "Anne", LastName: "Rice"},
}

func getAuthorById(id string) *Author {
	for _, author := range authors {
		if author.ID == id {
			return &author
		}
	}
	return nil
}

func TestGraphql(t *testing.T) {
	root, _ := os.Getwd()
	println(root)
	sdl, e := fileutil.ReadFile("resource/schema/examplev1_2.sdl")
	require.NoError(t, e)
	assert.NotEqual(t, sdl, "")
	QueryType, _ := Parsing(string(sdl),
		map[string]graphql.FieldResolveFn{
			"bookById": func(p graphql.ResolveParams) (interface{}, error) {
				return getById(p.Args["id"].(string)), nil
			}},
		map[string]graphql.FieldResolveFn{
			"Author": func(p graphql.ResolveParams) (interface{}, error) {
				if book, ok := p.Source.(*Book); ok {
					return getAuthorById(book.AuthorID), nil
				}
				return nil, nil
			},
		})
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query: QueryType,
	})

	query := `
query bookDetails {
  bookById(id: "book-1") {
    id
    name
    pageCount
    author {
      id
      firstName
      lastName
    }
  }
}
	`

	params := graphql.Params{Schema: schema, RequestString: query}
	r := graphql.Do(params)
	if len(r.Errors) > 0 {
		log.Fatalf("failed to execute graphql operation, errors: %+v", r.Errors)
	}
	rJSON, _ := json.Marshal(r)
	fmt.Printf("%s \n", rJSON) // {"data":{"hello":"world"}}
}
