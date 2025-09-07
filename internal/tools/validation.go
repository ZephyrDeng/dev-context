package tools

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Validator 参数验证器，提供统一的参数验证和错误处理
type Validator struct {
	dateRegex *regexp.Regexp
}

// NewValidator 创建新的参数验证器
func NewValidator() *Validator {
	return &Validator{
		dateRegex: regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
	}
}

// ValidationError 验证错误，包含详细的错误信息
type ValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Error 实现error接口
func (e ValidationError) Error() string {
	return fmt.Sprintf("验证失败 - 字段: %s, 值: %s, 错误: %s", e.Field, e.Value, e.Message)
}

// ValidationErrors 多个验证错误的集合
type ValidationErrors []ValidationError

// Error 实现error接口
func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// HasErrors 检查是否有验证错误
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// ValidateWeeklyNewsParams 验证周报新闻参数
func (v *Validator) ValidateWeeklyNewsParams(params WeeklyNewsParams) error {
	var errors ValidationErrors
	
	// 验证日期格式和逻辑
	if err := v.validateDateRange(params.StartDate, params.EndDate, "startDate", "endDate"); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrs...)
		} else {
			errors = append(errors, ValidationError{
				Field:   "dateRange",
				Message: err.Error(),
				Code:    "INVALID_DATE_RANGE",
			})
		}
	}
	
	// 验证分类
	if params.Category != "" {
		if err := v.validateCategory(params.Category); err != nil {
			errors = append(errors, ValidationError{
				Field:   "category",
				Value:   params.Category,
				Message: err.Error(),
				Code:    "INVALID_CATEGORY",
			})
		}
	}
	
	// 验证质量分数
	if err := v.validateQualityScore(params.MinQuality, "minQuality"); err != nil {
		errors = append(errors, ValidationError{
			Field:   "minQuality",
			Value:   fmt.Sprintf("%.2f", params.MinQuality),
			Message: err.Error(),
			Code:    "INVALID_QUALITY_SCORE",
		})
	}
	
	// 验证结果数量
	if err := v.validateResultCount(params.MaxResults, "maxResults", 1, 200); err != nil {
		errors = append(errors, ValidationError{
			Field:   "maxResults",
			Value:   fmt.Sprintf("%d", params.MaxResults),
			Message: err.Error(),
			Code:    "INVALID_RESULT_COUNT",
		})
	}
	
	// 验证格式
	if params.Format != "" {
		if err := v.validateFormat(params.Format); err != nil {
			errors = append(errors, ValidationError{
				Field:   "format",
				Value:   params.Format,
				Message: err.Error(),
				Code:    "INVALID_FORMAT",
			})
		}
	}
	
	// 验证排序方式
	if params.SortBy != "" {
		validSortBy := []string{"relevance", "quality", "date", "title"}
		if err := v.validateEnum(params.SortBy, validSortBy, "sortBy"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "sortBy",
				Value:   params.SortBy,
				Message: err.Error(),
				Code:    "INVALID_SORT_BY",
			})
		}
	}
	
	// 验证数据源
	if params.Sources != "" {
		if err := v.validateSources(params.Sources); err != nil {
			errors = append(errors, ValidationError{
				Field:   "sources",
				Value:   params.Sources,
				Message: err.Error(),
				Code:    "INVALID_SOURCES",
			})
		}
	}
	
	if errors.HasErrors() {
		return errors
	}
	
	return nil
}

// ValidateTopicSearchParams 验证主题搜索参数
func (v *Validator) ValidateTopicSearchParams(params TopicSearchParams) error {
	var errors ValidationErrors
	
	// 验证查询关键词（必需）
	if strings.TrimSpace(params.Query) == "" {
		errors = append(errors, ValidationError{
			Field:   "query",
			Value:   params.Query,
			Message: "查询关键词不能为空",
			Code:    "REQUIRED_FIELD_MISSING",
		})
	} else if len(strings.TrimSpace(params.Query)) < 2 {
		errors = append(errors, ValidationError{
			Field:   "query",
			Value:   params.Query,
			Message: "查询关键词至少需要2个字符",
			Code:    "QUERY_TOO_SHORT",
		})
	} else if len(params.Query) > 200 {
		errors = append(errors, ValidationError{
			Field:   "query",
			Value:   params.Query,
			Message: "查询关键词不能超过200个字符",
			Code:    "QUERY_TOO_LONG",
		})
	}
	
	// 验证编程语言
	if params.Language != "" {
		if err := v.validateLanguage(params.Language); err != nil {
			errors = append(errors, ValidationError{
				Field:   "language",
				Value:   params.Language,
				Message: err.Error(),
				Code:    "INVALID_LANGUAGE",
			})
		}
	}
	
	// 验证平台
	if params.Platform != "" {
		validPlatforms := []string{"github", "stackoverflow", "reddit", "dev.to"}
		if err := v.validateEnum(params.Platform, validPlatforms, "platform"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "platform",
				Value:   params.Platform,
				Message: err.Error(),
				Code:    "INVALID_PLATFORM",
			})
		}
	}
	
	// 验证排序方式
	if params.SortBy != "" {
		validSortBy := []string{"relevance", "date", "popularity", "stars"}
		if err := v.validateEnum(params.SortBy, validSortBy, "sortBy"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "sortBy",
				Value:   params.SortBy,
				Message: err.Error(),
				Code:    "INVALID_SORT_BY",
			})
		}
	}
	
	// 验证时间范围
	if params.TimeRange != "" {
		validTimeRanges := []string{"day", "week", "month", "year", "all"}
		if err := v.validateEnum(params.TimeRange, validTimeRanges, "timeRange"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "timeRange",
				Value:   params.TimeRange,
				Message: err.Error(),
				Code:    "INVALID_TIME_RANGE",
			})
		}
	}
	
	// 验证结果数量
	if err := v.validateResultCount(params.MaxResults, "maxResults", 1, 100); err != nil {
		errors = append(errors, ValidationError{
			Field:   "maxResults",
			Value:   fmt.Sprintf("%d", params.MaxResults),
			Message: err.Error(),
			Code:    "INVALID_RESULT_COUNT",
		})
	}
	
	// 验证格式
	if params.Format != "" {
		if err := v.validateFormat(params.Format); err != nil {
			errors = append(errors, ValidationError{
				Field:   "format",
				Value:   params.Format,
				Message: err.Error(),
				Code:    "INVALID_FORMAT",
			})
		}
	}
	
	// 验证相关性分数
	if err := v.validateQualityScore(params.MinScore, "minScore"); err != nil {
		errors = append(errors, ValidationError{
			Field:   "minScore",
			Value:   fmt.Sprintf("%.2f", params.MinScore),
			Message: err.Error(),
			Code:    "INVALID_SCORE",
		})
	}
	
	// 验证搜索类型
	if params.SearchType != "" {
		validSearchTypes := []string{"discussions", "repositories", "articles", "all"}
		if err := v.validateEnum(params.SearchType, validSearchTypes, "searchType"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "searchType",
				Value:   params.SearchType,
				Message: err.Error(),
				Code:    "INVALID_SEARCH_TYPE",
			})
		}
	}
	
	if errors.HasErrors() {
		return errors
	}
	
	return nil
}

// ValidateTrendingReposParams 验证热门仓库参数
func (v *Validator) ValidateTrendingReposParams(params TrendingReposParams) error {
	var errors ValidationErrors
	
	// 验证编程语言
	if params.Language != "" {
		if err := v.validateLanguage(params.Language); err != nil {
			errors = append(errors, ValidationError{
				Field:   "language",
				Value:   params.Language,
				Message: err.Error(),
				Code:    "INVALID_LANGUAGE",
			})
		}
	}
	
	// 验证时间范围
	if params.TimeRange != "" {
		validTimeRanges := []string{"daily", "weekly", "monthly"}
		if err := v.validateEnum(params.TimeRange, validTimeRanges, "timeRange"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "timeRange",
				Value:   params.TimeRange,
				Message: err.Error(),
				Code:    "INVALID_TIME_RANGE",
			})
		}
	}
	
	// 验证最小星标数
	if params.MinStars < 0 {
		errors = append(errors, ValidationError{
			Field:   "minStars",
			Value:   fmt.Sprintf("%d", params.MinStars),
			Message: "最小星标数不能为负数",
			Code:    "INVALID_MIN_STARS",
		})
	} else if params.MinStars > 100000 {
		errors = append(errors, ValidationError{
			Field:   "minStars",
			Value:   fmt.Sprintf("%d", params.MinStars),
			Message: "最小星标数不能超过100000",
			Code:    "MIN_STARS_TOO_HIGH",
		})
	}
	
	// 验证结果数量
	if err := v.validateResultCount(params.MaxResults, "maxResults", 1, 100); err != nil {
		errors = append(errors, ValidationError{
			Field:   "maxResults",
			Value:   fmt.Sprintf("%d", params.MaxResults),
			Message: err.Error(),
			Code:    "INVALID_RESULT_COUNT",
		})
	}
	
	// 验证分类
	if params.Category != "" {
		validCategories := []string{"framework", "library", "tool", "example"}
		if err := v.validateEnum(params.Category, validCategories, "category"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "category",
				Value:   params.Category,
				Message: err.Error(),
				Code:    "INVALID_CATEGORY",
			})
		}
	}
	
	// 验证排序方式
	if params.SortBy != "" {
		validSortBy := []string{"stars", "forks", "updated", "trending"}
		if err := v.validateEnum(params.SortBy, validSortBy, "sortBy"); err != nil {
			errors = append(errors, ValidationError{
				Field:   "sortBy",
				Value:   params.SortBy,
				Message: err.Error(),
				Code:    "INVALID_SORT_BY",
			})
		}
	}
	
	// 验证格式
	if params.Format != "" {
		if err := v.validateFormat(params.Format); err != nil {
			errors = append(errors, ValidationError{
				Field:   "format",
				Value:   params.Format,
				Message: err.Error(),
				Code:    "INVALID_FORMAT",
			})
		}
	}
	
	if errors.HasErrors() {
		return errors
	}
	
	return nil
}

// 辅助验证方法

// validateDateRange 验证日期范围
func (v *Validator) validateDateRange(startDate, endDate, startField, endField string) error {
	var errors ValidationErrors
	
	var start, end time.Time
	var err error
	
	// 验证开始日期格式
	if startDate != "" {
		if !v.dateRegex.MatchString(startDate) {
			errors = append(errors, ValidationError{
				Field:   startField,
				Value:   startDate,
				Message: "日期格式必须为YYYY-MM-DD",
				Code:    "INVALID_DATE_FORMAT",
			})
		} else {
			start, err = time.Parse("2006-01-02", startDate)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   startField,
					Value:   startDate,
					Message: "日期解析失败",
					Code:    "DATE_PARSE_ERROR",
				})
			}
		}
	}
	
	// 验证结束日期格式
	if endDate != "" {
		if !v.dateRegex.MatchString(endDate) {
			errors = append(errors, ValidationError{
				Field:   endField,
				Value:   endDate,
				Message: "日期格式必须为YYYY-MM-DD",
				Code:    "INVALID_DATE_FORMAT",
			})
		} else {
			end, err = time.Parse("2006-01-02", endDate)
			if err != nil {
				errors = append(errors, ValidationError{
					Field:   endField,
					Value:   endDate,
					Message: "日期解析失败",
					Code:    "DATE_PARSE_ERROR",
				})
			}
		}
	}
	
	// 验证日期逻辑
	if !start.IsZero() && !end.IsZero() {
		if start.After(end) {
			errors = append(errors, ValidationError{
				Field:   "dateRange",
				Value:   fmt.Sprintf("%s to %s", startDate, endDate),
				Message: "开始日期不能晚于结束日期",
				Code:    "INVALID_DATE_ORDER",
			})
		}
		
		// 检查时间范围是否过长
		if end.Sub(start) > 90*24*time.Hour {
			errors = append(errors, ValidationError{
				Field:   "dateRange",
				Value:   fmt.Sprintf("%s to %s", startDate, endDate),
				Message: "时间范围不能超过90天",
				Code:    "DATE_RANGE_TOO_LONG",
			})
		}
	}
	
	// 验证日期不能是未来
	now := time.Now()
	if !start.IsZero() && start.After(now.Add(24*time.Hour)) {
		errors = append(errors, ValidationError{
			Field:   startField,
			Value:   startDate,
			Message: "开始日期不能是未来日期",
			Code:    "FUTURE_DATE_NOT_ALLOWED",
		})
	}
	
	if !end.IsZero() && end.After(now.Add(24*time.Hour)) {
		errors = append(errors, ValidationError{
			Field:   endField,
			Value:   endDate,
			Message: "结束日期不能是未来日期",
			Code:    "FUTURE_DATE_NOT_ALLOWED",
		})
	}
	
	if errors.HasErrors() {
		return errors
	}
	
	return nil
}

// validateCategory 验证分类
func (v *Validator) validateCategory(category string) error {
	validCategories := []string{
		"react", "vue", "angular", "nodejs", "typescript", "javascript", 
		"css", "testing", "webpack", "framework", "library", "tool", "example",
	}
	
	category = strings.ToLower(strings.TrimSpace(category))
	for _, valid := range validCategories {
		if category == valid {
			return nil
		}
	}
	
	return fmt.Errorf("分类必须是以下之一: %v", validCategories)
}

// validateLanguage 验证编程语言
func (v *Validator) validateLanguage(language string) error {
	validLanguages := []string{
		"javascript", "typescript", "python", "java", "go", "rust", "c++", "c#", 
		"php", "ruby", "swift", "kotlin", "dart", "html", "css", "scss", "sass",
		"vue", "jsx", "tsx",
	}
	
	language = strings.ToLower(strings.TrimSpace(language))
	for _, valid := range validLanguages {
		if language == valid {
			return nil
		}
	}
	
	return fmt.Errorf("编程语言必须是以下之一: %v", validLanguages)
}

// validateQualityScore 验证质量分数
func (v *Validator) validateQualityScore(score float64, fieldName string) error {
	if score < 0.0 || score > 1.0 {
		return fmt.Errorf("%s 必须在0.0-1.0之间", fieldName)
	}
	return nil
}

// validateResultCount 验证结果数量
func (v *Validator) validateResultCount(count int, fieldName string, min, max int) error {
	if count < min || count > max {
		return fmt.Errorf("%s 必须在%d-%d之间", fieldName, min, max)
	}
	return nil
}

// validateFormat 验证输出格式
func (v *Validator) validateFormat(format string) error {
	validFormats := []string{"json", "markdown", "text"}
	return v.validateEnum(format, validFormats, "format")
}

// validateEnum 验证枚举值
func (v *Validator) validateEnum(value string, validValues []string, fieldName string) error {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, valid := range validValues {
		if value == valid {
			return nil
		}
	}
	return fmt.Errorf("%s 必须是以下之一: %v", fieldName, validValues)
}

// validateSources 验证数据源
func (v *Validator) validateSources(sources string) error {
	validSources := []string{"dev.to", "hackernews", "reddit", "medium", "github"}
	
	sourceList := strings.Split(sources, ",")
	for _, source := range sourceList {
		source = strings.ToLower(strings.TrimSpace(source))
		if source == "" {
			continue
		}
		
		found := false
		for _, valid := range validSources {
			if source == valid {
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("数据源 '%s' 无效，有效的数据源: %v", source, validSources)
		}
	}
	
	return nil
}

// SanitizeInput 清理输入，防止XSS和注入攻击
func (v *Validator) SanitizeInput(input string) string {
	// 移除潜在的恶意字符
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")
	input = strings.ReplaceAll(input, "&", "&amp;")
	
	// 移除控制字符
	input = regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(input, "")
	
	return strings.TrimSpace(input)
}

// ValidateAndSanitizeQuery 验证并清理查询字符串
func (v *Validator) ValidateAndSanitizeQuery(query string) (string, error) {
	// 清理输入
	query = v.SanitizeInput(query)
	
	// 验证长度
	if len(query) < 2 {
		return "", fmt.Errorf("查询关键词至少需要2个字符")
	}
	
	if len(query) > 200 {
		return "", fmt.Errorf("查询关键词不能超过200个字符")
	}
	
	// 验证字符集（只允许字母、数字、空格和常用符号）
	validChars := regexp.MustCompile(`^[a-zA-Z0-9\s\-_\.\/\+\#\@]+$`)
	if !validChars.MatchString(query) {
		return "", fmt.Errorf("查询关键词包含无效字符")
	}
	
	return query, nil
}

// GetValidationErrorResponse 获取格式化的验证错误响应
func (v *Validator) GetValidationErrorResponse(err error) map[string]interface{} {
	response := map[string]interface{}{
		"success": false,
		"error":   "参数验证失败",
	}
	
	if validationErrs, ok := err.(ValidationErrors); ok {
		var details []map[string]interface{}
		for _, validationErr := range validationErrs {
			details = append(details, map[string]interface{}{
				"field":   validationErr.Field,
				"value":   validationErr.Value,
				"message": validationErr.Message,
				"code":    validationErr.Code,
			})
		}
		response["details"] = details
	} else {
		response["message"] = err.Error()
	}
	
	return response
}