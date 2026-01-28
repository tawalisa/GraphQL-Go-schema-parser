package schemaParser

import (
	"fmt"
	"regexp"
	"strings"
)

// GoTypeGenerator 用于根据GraphQL Schema生成Go结构体
type GoTypeGenerator struct {
	SchemaContent string
}

// NewGoTypeGenerator 创建一个新的Go类型生成器
func NewGoTypeGenerator(schemaContent string) *GoTypeGenerator {
	return &GoTypeGenerator{
		SchemaContent: schemaContent,
	}
}

// GenerateGoStructs 根据GraphQL Schema生成Go结构体定义
func (g *GoTypeGenerator) GenerateGoStructs() (map[string]string, error) {
	types, err := g.parseTypes()
	if err != nil {
		return nil, err
	}

	goStructs := make(map[string]string)

	for typeName, fields := range types {
		goStructs[typeName] = g.generateStruct(typeName, fields)
	}

	return goStructs, nil
}

// parseTypes 解析GraphQL Schema中的类型定义
func (g *GoTypeGenerator) parseTypes() (map[string]map[string]string, error) {
	types := make(map[string]map[string]string)
	lines := strings.Split(g.SchemaContent, "\n")

	var currentTypeName string
	var currentFields map[string]string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// 跳过空行和注释
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// 检查是否是类型定义开始
		if strings.HasPrefix(trimmedLine, "type ") {
			// 保存上一个类型（如果有）
			if currentTypeName != "" && currentFields != nil {
				types[currentTypeName] = currentFields
			}

			// 开始新类型定义
			typeDef := trimmedLine[5:] // 移除 "type " 前缀
			currentTypeName = g.extractTypeName(typeDef)
			currentFields = make(map[string]string)
		} else if strings.Contains(trimmedLine, "{") {
			// 忽略 { 行
			continue
		} else if strings.Contains(trimmedLine, "}") {
			// 结束当前类型定义
			if currentTypeName != "" && currentFields != nil {
				types[currentTypeName] = currentFields
				currentTypeName = ""
				currentFields = nil
			}
		} else if currentTypeName != "" && strings.Contains(trimmedLine, ":") {
			// 解析字段定义
			fieldName, fieldType := g.splitLastColonPair(trimmedLine)
			fieldName = strings.TrimSpace(fieldName)
			fieldType = strings.TrimSpace(fieldType)

			// 移除可能的非空标记
			fieldType = strings.ReplaceAll(fieldType, "!", "")

			currentFields[fieldName] = fieldType
		}
	}

	// 保存最后一个类型（如果有）
	if currentTypeName != "" && currentFields != nil {
		types[currentTypeName] = currentFields
	}

	return types, nil
}

// generateStruct 生成Go结构体定义
func (g *GoTypeGenerator) generateStruct(typeName string, fields map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("type %s struct {\n", typeName))

	for fieldName, fieldType := range fields {
		// 提取字段名（去掉参数部分）
		cleanFieldName := fieldName
		if idx := strings.Index(cleanFieldName, "("); idx != -1 {
			cleanFieldName = cleanFieldName[:idx]
		}
		cleanFieldName = strings.TrimSpace(cleanFieldName)

		goFieldType := g.convertGraphQLTypeToGo(fieldType)
		jsonTag := fmt.Sprintf("`json:\"%s\"`", cleanFieldName)
		sb.WriteString(fmt.Sprintf("\t%s %s %s\n", g.toPascalCase(cleanFieldName), goFieldType, jsonTag))
	}

	sb.WriteString("}")

	return sb.String()
}

// convertGraphQLTypeToGo 将GraphQL类型转换为Go类型
func (g *GoTypeGenerator) convertGraphQLTypeToGo(graphQLType string) string {
	// 检查是否是数组类型
	arrayRegex := regexp.MustCompile(`^\[(.*)\]$`)
	matches := arrayRegex.FindStringSubmatch(graphQLType)
	if len(matches) > 1 {
		elementType := g.convertGraphQLTypeToGo(matches[1])
		return fmt.Sprintf("[]%s", elementType)
	}

	// 基础类型映射
	switch graphQLType {
	case "String":
		return "string"
	case "Int":
		return "int"
	case "Float", "BigDecimal":
		return "float64"
	case "Boolean":
		return "bool"
	case "ID":
		return "string"
	default:
		// 假设是自定义类型
		return graphQLType
	}
}

// toPascalCase 将字符串转换为帕斯卡命名法
func (g *GoTypeGenerator) toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		parts[i] = strings.Title(strings.ToLower(part))
	}
	return strings.Join(parts, "")
}

// splitLastColonPair 按最后的冒号分割字符串
func (g *GoTypeGenerator) splitLastColonPair(s string) (string, string) {
	lastIndex := strings.LastIndex(s, ":")
	if lastIndex != -1 {
		return strings.TrimSpace(s[:lastIndex]), strings.TrimSpace(s[lastIndex+1:])
	}
	return s, ""
}

// extractTypeName 提取类型名称
func (g *GoTypeGenerator) extractTypeName(def string) string {
	parts := strings.Fields(def)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// GenerateResolvers 生成resolver函数模板
func (g *GoTypeGenerator) GenerateResolvers() (string, error) {
	types, err := g.parseTypes()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("// Generated resolver functions\n")
	sb.WriteString("package schemaParser\n\n")
	sb.WriteString("import \"github.com/graphql-go/graphql\"\n\n")

	// 为每个类型生成resolver函数
	for typeName, fields := range types {
		if typeName == "Query" {
			// 为Query类型中的每个字段生成resolver
			for fieldName := range fields {
				// 提取干净的字段名（去除参数）
				cleanFieldName := fieldName
				if idx := strings.Index(cleanFieldName, "("); idx != -1 {
					cleanFieldName = cleanFieldName[:idx]
				}
				cleanFieldName = strings.TrimSpace(cleanFieldName)

				sb.WriteString(fmt.Sprintf("// Resolve%s resolves the %s field\n", g.toPascalCase(cleanFieldName), cleanFieldName))
				sb.WriteString(fmt.Sprintf("func Resolve%s(p graphql.ResolveParams) (interface{}, error) {\n", g.toPascalCase(cleanFieldName)))
				sb.WriteString("\t// TODO: Implement resolver logic\n")
				sb.WriteString("\treturn nil, nil\n")
				sb.WriteString("}\n\n")
			}
		} else {
			// 为其他类型生成resolver
			sb.WriteString(fmt.Sprintf("// Resolve%s resolves fields for type %s\n", typeName, typeName))
			sb.WriteString(fmt.Sprintf("func Resolve%s(p graphql.ResolveParams) (interface{}, error) {\n", typeName))
			sb.WriteString("\t// TODO: Implement resolver logic\n")
			sb.WriteString("\treturn nil, nil\n")
			sb.WriteString("}\n\n")
		}
	}

	return sb.String(), nil
}
