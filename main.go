package main

import (
	"fmt"
	"log"

	"github.com/graphql-go/graphql"
	"graphql-go-schema-parser/schemaParser"
	fileutil "graphql-go-schema-parser/util"
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

func main() {
	fmt.Println("GraphQL Schema Parser Demo")

	// 读取GraphQL Schema文件
	sdl, err := fileutil.ReadFile("resource/schema/examplev1.sdl")
	if err != nil {
		log.Fatal("Error reading schema file:", err)
	}

	fmt.Println("Schema content:")
	fmt.Println(string(sdl))

	// 解析GraphQL Schema
	queryType, err := schemaParser.Parsing(string(sdl),
		map[string]graphql.FieldResolveFn{
			"bookById": func(p graphql.ResolveParams) (interface{}, error) {
				id, ok := p.Args["id"].(string)
				if !ok {
					return nil, fmt.Errorf("invalid id argument")
				}
				return getById(id), nil
			},
		},
		map[string]graphql.FieldResolveFn{
			"author": func(p graphql.ResolveParams) (interface{}, error) {
				if book, ok := p.Source.(*Book); ok {
					return getAuthorById(book.AuthorID), nil
				}
				return nil, nil
			},
		})

	if err != nil {
		log.Fatal("Error parsing schema:", err)
	}

	fmt.Println("Successfully parsed schema")

	// 尝试输出一些调试信息
	fmt.Printf("Query type created: %s\n", queryType.Name())

	// 创建GraphQL Schema
	config := graphql.SchemaConfig{
		Query: queryType,
	}

	schema, err := graphql.NewSchema(config)
	if err != nil {
		log.Fatal("Error creating schema:", err)
	}

	// 执行查询
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

	params := graphql.Params{
		Schema:        schema,
		RequestString: query,
	}
	result := graphql.Do(params)

	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, err := range result.Errors {
			fmt.Printf("  %s\n", err)
		}
	} else {
		fmt.Printf("Query Result: %+v\n", result)
	}

	// 演示Go结构体生成
	fmt.Println("\nGenerating Go structs from GraphQL schema...")
	generator := schemaParser.NewGoTypeGenerator(string(sdl))
	goStructs, err := generator.GenerateGoStructs()
	if err != nil {
		log.Fatal("Error generating Go structs:", err)
	}

	for typeName, structDef := range goStructs {
		fmt.Printf("\nGenerated struct for %s:\n%s\n", typeName, structDef)
	}

	// 演示resolver生成
	fmt.Println("\nGenerating resolver functions...")
	resolvers, err := generator.GenerateResolvers()
	if err != nil {
		log.Fatal("Error generating resolvers:", err)
	}
	fmt.Printf("\nGenerated resolvers:\n%s\n", resolvers)
}
