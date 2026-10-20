package pagination

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Значения по умолчанию и ограничения для пагинации.
const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

var (
	ErrPageInvalid     = errors.New("номер страницы 'page' должен быть целым числом >= 1")
	ErrPageSizeInvalid = errors.New("размер страницы 'page_size' должен быть целым числом >= 1")
	ErrPageSizeExceeds = fmt.Errorf("размер страницы 'page_size' не может превышать %d", MaxPageSize)
)

// Pagination содержит разобранные параметры постраничной навигации.
type Pagination struct {
	Page     int `json:"page" form:"page"`
	PageSize int `json:"page_size" form:"page_size"`
}

// Offset вычисляет смещение для SQL-запроса (OFFSET).
func (p Pagination) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Limit возвращает лимит записей для SQL-запроса (LIMIT).
func (p Pagination) Limit() int {
	if p.PageSize <= 0 {
		return DefaultPageSize
	}
	return p.PageSize
}

// PagedResponse представляет обобщенную структуру ответа со списком и метаданными пагинации.
type PagedResponse[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// NewPagedResponse создаёт новый PagedResponse, гарантируя, что Items никогда не равен nil (будет пустой массив [] в JSON).
func NewPagedResponse[T any](items []T, total int, page int, pageSize int) PagedResponse[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return PagedResponse[T]{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// Parse извлекает и валидирует параметры page и page_size из query-параметров запроса.
// Если параметры отсутствуют — применяются значения по умолчанию: page=1, page_size=20.
// Если page_size > 100 или page < 1 — возвращается ошибка.
func Parse(c *gin.Context) (Pagination, error) {
	page := DefaultPage
	pageSize := DefaultPageSize

	if pageStr := strings.TrimSpace(c.Query("page")); pageStr != "" {
		val, err := strconv.Atoi(pageStr)
		if err != nil || val < 1 {
			return Pagination{}, ErrPageInvalid
		}
		page = val
	}

	if pageSizeStr := strings.TrimSpace(c.Query("page_size")); pageSizeStr != "" {
		val, err := strconv.Atoi(pageSizeStr)
		if err != nil || val < 1 {
			return Pagination{}, ErrPageSizeInvalid
		}
		if val > MaxPageSize {
			return Pagination{}, ErrPageSizeExceeds
		}
		pageSize = val
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ParseOrAbort извлекает параметры пагинации или автоматически прерывает запрос с кодом 400 Bad Request при ошибке.
func ParseOrAbort(c *gin.Context) (Pagination, bool) {
	p, err := Parse(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return Pagination{}, false
	}
	return p, true
}
