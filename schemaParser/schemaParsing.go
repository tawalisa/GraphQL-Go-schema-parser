package schemaParser

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/graphql-go/graphql"
)

type GraphQLType struct {
	Query *graphql.Object
}

type GraphQLParsingType struct {
	name     string
	typeName string
	fields   map[string]GraphQLParsingField
}

type GraphqlParingParam struct {
	name  string
	typeP string
}

type GraphQLParsingField struct {
	name     string
	typeName string
	params   []GraphqlParingParam
}

// 解析 GraphQL Schema 字符串并生成 QueryType
// 这是主要入口函数
func Parsing(sdlContent string, queryFucMap map[string]graphql.FieldResolveFn, schemaFucMap map[string]graphql.FieldResolveFn) (*graphql.Object, error) {
	// 预处理 SDL 内容，标准化格式
	normalizedContent := normalizeSDL(sdlContent)

	// 解析类型定义
	types, err := parseTypes(normalizedContent)
	if err != nil {
		return nil, err
	}

	// 创建类型映射
	typeMap := make(map[string]GraphQLParsingType)
	for _, t := range types {
		if t.typeName != "enum" && t.typeName != "union" {
			typeMap[t.name] = t
		}
	}

	// 创建GraphQL对象映射，使用懒加载避免循环依赖
	graphqlObjMap := make(map[string]*graphql.Object)

	// 为所有类型创建占位符，但暂不填充字段
	for typeName := range typeMap {
		graphqlObjMap[typeName] = nil
	}

	// 为每个类型创建完整定义（使用缓存/懒加载）
	for typeName := range typeMap {
		getOrCreateGraphqlTypeWrapped(typeName, typeMap, graphqlObjMap, queryFucMap, schemaFucMap)
	}

	// 获取并返回 Query 类型
	queryObj, exists := graphqlObjMap["Query"]
	if !exists {
		queryObj, exists = graphqlObjMap["QueryType"] // 兼容性处理
		if !exists {
			return nil, fmt.Errorf("Query type not found in schema")
		}
	}

	return queryObj, nil
}

// 包装函数，用于创建GraphQL类型
func getOrCreateGraphqlTypeWrapped(name string, typeMap map[string]GraphQLParsingType, objMap map[string]*graphql.Object, queryFucMap map[string]graphql.FieldResolveFn, schemaFucMap map[string]graphql.FieldResolveFn) *graphql.Object {
	// 如果已经创建，直接返回
	if obj, exists := objMap[name]; exists && obj != nil {
		return obj
	}

	// 获取类型定义
	typeDef, exists := typeMap[name]
	if !exists {
		panic(fmt.Sprintf("type '%s' not found in schema", name))
	}

	// 先创建一个临时对象以处理循环依赖
	tempObj := graphql.NewObject(graphql.ObjectConfig{
		Name:   typeDef.name,
		Fields: graphql.Fields{},
	})
	objMap[name] = tempObj

	// 创建字段
	fields := make(graphql.Fields)

	for fieldName, fieldDef := range typeDef.fields {
		// 创建字段配置
		fieldConfig := &graphql.Field{
			Type: getGraphqlTypeRecursive(fieldDef.typeName, typeMap, objMap, queryFucMap, schemaFucMap),
		}

		// 添加参数（如果存在）
		if len(fieldDef.params) > 0 {
			fieldConfig.Args = graphQLArgs(fieldDef.params, objMap)
		}

		// 设置resolver函数
		if typeDef.name == "Query" {
			// Query类型的字段使用queryFucMap
			if resolver, exists := queryFucMap[fieldDef.name]; exists {
				fieldConfig.Resolve = resolver
			} else {
				// 默认resolver
				fieldConfig.Resolve = defaultFieldResolver
			}
		} else {
			// 其他类型的字段优先使用字段名作为resolver键
			if resolver, exists := schemaFucMap[fieldDef.name]; exists {
				fieldConfig.Resolve = resolver
			} else if resolver, exists := schemaFucMap[fieldDef.typeName]; exists {
				// 如果没有字段名resolver，尝试类型名resolver
				fieldConfig.Resolve = resolver
			} else {
				// 默认resolver
				fieldConfig.Resolve = defaultFieldResolver
			}
		}

		fields[fieldName] = fieldConfig
	}

	// 更新对象，设置实际字段
	finalObj := graphql.NewObject(graphql.ObjectConfig{
		Name:   typeDef.name,
		Fields: fields,
	})
	objMap[name] = finalObj

	return finalObj
}

// 递归获取GraphQL类型（处理依赖关系）
func getGraphqlTypeRecursive(name string, typeMap map[string]GraphQLParsingType, objMap map[string]*graphql.Object, queryFucMap map[string]graphql.FieldResolveFn, schemaFucMap map[string]graphql.FieldResolveFn) graphql.Output {
	name = strings.TrimSpace(name)

	// 检查是否为列表类型
	isList := strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]")
	var innerName string
	if isList {
		innerName = strings.Trim(name, "[]")
	} else {
		innerName = name
	}

	// 检查是否为非空类型
	isNonNull := strings.HasSuffix(innerName, "!")
	if isNonNull {
		innerName = strings.TrimSuffix(innerName, "!")
	}

	var result graphql.Output
	// 首先检查是否为基础类型
	switch innerName {
	case "ID":
		result = graphql.ID
	case "String":
		result = graphql.String
	case "Int":
		result = graphql.Int
	case "Float", "BigDecimal":
		result = graphql.Float
	case "Boolean":
		result = graphql.Boolean
	case "DateTime":
		result = graphql.DateTime
	default:
		// 检查是否已在objMap中创建
		if obj, exists := objMap[innerName]; exists && obj != nil {
			result = obj
		} else {
			// 如果类型尚未创建，先创建它
			if _, exists := typeMap[innerName]; exists {
				result = getOrCreateGraphqlTypeWrapped(innerName, typeMap, objMap, queryFucMap, schemaFucMap)
			} else {
				panic(fmt.Sprintf("type '%s' not found in schema", innerName))
			}
		}
	}

	// 应用列表包装
	if isList {
		result = graphql.NewList(result)
	}

	// 应用非空包装
	if isNonNull {
		result = graphql.NewNonNull(result)
	}

	return result
}

// 获取GraphQL类型
func getGraphqlType(name string, objMap map[string]*graphql.Object) graphql.Output {
	name = strings.TrimSpace(name)

	// 检查是否为列表类型
	isList := strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]")
	var innerName string
	if isList {
		innerName = strings.Trim(name, "[]")
	} else {
		innerName = name
	}

	// 检查是否为非空类型
	isNonNull := strings.HasSuffix(innerName, "!")
	if isNonNull {
		innerName = strings.TrimSuffix(innerName, "!")
	}

	var result graphql.Output
	// 首先检查是否为基础类型
	switch innerName {
	case "ID":
		result = graphql.ID
	case "String":
		result = graphql.String
	case "Int":
		result = graphql.Int
	case "Float", "BigDecimal":
		result = graphql.Float
	case "Boolean":
		result = graphql.Boolean
	case "DateTime":
		result = graphql.DateTime
	default:
		// 处理自定义类型
		obj, exists := objMap[innerName]
		if !exists {
			panic(fmt.Sprintf("type '%s' not found in schema", innerName))
		}
		result = obj
	}

	// 应用列表包装
	if isList {
		result = graphql.NewList(result)
	}

	// 应用非空包装
	if isNonNull {
		result = graphql.NewNonNull(result)
	}

	return result
}

// 规范化SDL内容
func normalizeSDL(content string) string {
	// 标准化换行符
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// 使用正则表达式提取所有类型定义，正确处理嵌套括号
	re := regexp.MustCompile(`(?s)(type|interface|enum|union)\s+(\w+)\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	var normalized strings.Builder
	processedTypes := make(map[string]bool)

	// 重新组织类型定义，确保格式正确
	for _, match := range matches {
		if len(match) >= 4 {
			typeDef := fmt.Sprintf("\n%s %s {\n%s\n}\n", match[1], match[2], strings.TrimSpace(match[3]))
			normalized.WriteString(typeDef)
			processedTypes[match[2]] = true
		}
	}

	return normalized.String()
}

// 解析类型定义
func parseTypes(content string) ([]GraphQLParsingType, error) {
	var types []GraphQLParsingType

	// 匹配类型定义的正则表达式，正确处理嵌套
	re := regexp.MustCompile(`(?s)(type|interface|enum|union)\s+(\w+)\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			typeKeyword := match[1]
			typeName := match[2]
			typeBody := match[3]

			// 解析类型体中的字段
			fields := parseFields(typeBody)

			types = append(types, GraphQLParsingType{
				name:     typeName,
				typeName: typeKeyword,
				fields:   fields,
			})
		}
	}

	return types, nil
}

// 解析字段定义
func parseFields(body string) map[string]GraphQLParsingField {
	fields := make(map[string]GraphQLParsingField)

	// 按行分割处理
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// 跳过空行和注释
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, "//") {
			continue
		}

		// 查找字段定义（包含冒号）
		if strings.Contains(trimmedLine, ":") {
			fieldName, fieldType := splitFieldDefinition(trimmedLine)

			if fieldName != "" && fieldType != "" {
				// 解析参数（如果存在）
				fieldPart, paramsPart := extractParamsFromField(fieldName)

				field := GraphQLParsingField{
					name:     fieldPart,
					typeName: fieldType,
					params:   paramsPart,
				}

				fields[fieldPart] = field
			}
		}
	}

	return fields
}

// 智能分割字段定义，处理参数和返回类型
func splitFieldDefinition(fieldLine string) (string, string) {
	fieldLine = strings.TrimSpace(fieldLine)
	if fieldLine == "" {
		return "", ""
	}

	// 正确处理参数括号，找到不在括号内的冒号
	parenDepth := 0
	colonPos := -1

	for i, char := range fieldLine {
		switch char {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case ':':
			if parenDepth == 0 {
				colonPos = i
				break
			}
		}
	}

	if colonPos != -1 {
		fieldPart := strings.TrimSpace(fieldLine[:colonPos])
		typePart := strings.TrimSpace(fieldLine[colonPos+1:])

		return fieldPart, typePart
	}

	return fieldLine, ""
}

// 从字段定义中提取参数
func extractParamsFromField(fieldDef string) (string, []GraphqlParingParam) {
	openParen := strings.Index(fieldDef, "(")
	closeParen := strings.LastIndex(fieldDef, ")")

	if openParen != -1 && closeParen != -1 && closeParen > openParen {
		fieldName := strings.TrimSpace(fieldDef[:openParen])
		paramsStr := fieldDef[openParen+1 : closeParen]

		var params []GraphqlParingParam
		if paramsStr != "" {
			// 简单分割参数（这里可以进一步优化以处理嵌套结构）
			paramDefs := strings.Split(paramsStr, ",")
			for _, paramDef := range paramDefs {
				paramDef = strings.TrimSpace(paramDef)
				if paramDef != "" {
					parts := strings.Split(paramDef, ":")
					if len(parts) == 2 {
						param := GraphqlParingParam{
							name:  strings.TrimSpace(parts[0]),
							typeP: strings.TrimSpace(parts[1]),
						}
						params = append(params, param)
					}
				}
			}
		}

		return fieldName, params
	}

	return strings.TrimSpace(fieldDef), nil
}

// 创建GraphQL参数
func graphQLArgs(params []GraphqlParingParam, objMap map[string]*graphql.Object) graphql.FieldConfigArgument {
	args := make(graphql.FieldConfigArgument)
	for _, p := range params {
		args[p.name] = &graphql.ArgumentConfig{
			Type: getGraphqlType(p.typeP, objMap),
		}
	}
	return args
}

// 默认字段解析器
func defaultFieldResolver(p graphql.ResolveParams) (interface{}, error) {
	// 使用反射从源对象中获取字段值
	sourceValue := reflect.ValueOf(p.Source)

	// 处理指针
	if sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}

	// 确保是结构体
	if sourceValue.Kind() != reflect.Struct {
		return nil, nil
	}

	// 获取字段名
	fieldName := p.Info.FieldName

	// 首先尝试直接匹配字段名
	fieldValue := sourceValue.FieldByName(fieldName)
	if fieldValue.IsValid() && fieldValue.CanInterface() {
		return fieldValue.Interface(), nil
	}

	// 尝试转换GraphQL字段名到Go字段名（例如：firstName -> FirstName）
	goFieldName := convertGraphQLNameToGo(fieldName)
	fieldValue = sourceValue.FieldByName(goFieldName)
	if fieldValue.IsValid() && fieldValue.CanInterface() {
		return fieldValue.Interface(), nil
	}

	// 尝试更复杂的转换（驼峰命名）
	goFieldName2 := toPascalCase(fieldName)
	if goFieldName2 != goFieldName {
		fieldValue = sourceValue.FieldByName(goFieldName2)
		if fieldValue.IsValid() && fieldValue.CanInterface() {
			return fieldValue.Interface(), nil
		}
	}

	return nil, nil
}

// 将GraphQL字段名转换为Go字段名
func convertGraphQLNameToGo(graphQLName string) string {
	// 简单的驼峰转换：firstName -> FirstName
	if len(graphQLName) == 0 {
		return graphQLName
	}
	return string(unicode.ToUpper(rune(graphQLName[0]))) + graphQLName[1:]
}

// 另一种驼峰转换方法
func toPascalCase(s string) string {
	// 将下划线命名或驼峰命名转换为PascalCase
	if s == "" {
		return s
	}

	// 如果已经是PascalCase，直接返回
	if s[0] >= 'A' && s[0] <= 'Z' {
		return s
	}

	// 将驼峰命名转换为PascalCase
	result := make([]rune, 0, len(s))
	upperNext := true

	for _, r := range s {
		if r == '_' || r == ' ' {
			upperNext = true
		} else if upperNext {
			result = append(result, unicode.ToUpper(r))
			upperNext = false
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

// 简单的Unicode大写转换
func unicodeToUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}
