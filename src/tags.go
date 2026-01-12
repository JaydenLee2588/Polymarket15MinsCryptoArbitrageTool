package src

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// GetTags 获取带有可选过滤和分页的标签
// 参考: https://gamma-api.polymarket.com/tags
func (c *Client) GetTags(ctx context.Context, params *GetTagsParams) ([]Tag, error) {
	path := "/tags?"

	urlParams := url.Values{}

	if params != nil {
		if params.Limit > 0 {
			urlParams.Add("limit", fmt.Sprintf("%d", params.Limit))
		}
		if params.Offset > 0 {
			urlParams.Add("offset", fmt.Sprintf("%d", params.Offset))
		}
		if params.Order != "" {
			urlParams.Add("order", params.Order)
		}
		if params.Ascending != nil {
			urlParams.Add("ascending", fmt.Sprintf("%t", *params.Ascending))
		}
		if params.IncludeTemplate != nil {
			urlParams.Add("include_template", fmt.Sprintf("%t", *params.IncludeTemplate))
		}
		if params.IsCarousel != nil {
			urlParams.Add("is_carousel", fmt.Sprintf("%t", *params.IsCarousel))
		}
	}

	path += urlParams.Encode()

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var tags []Tag
	if err := json.Unmarshal(respBody, &tags); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return tags, nil
}

// GetTagByID 通过 ID 获取特定标签
// 参考: https://gamma-api.polymarket.com/tags/{id}
func (c *Client) GetTagByID(ctx context.Context, tagID string, params *GetTagByIDQueryParams) (*Tag, error) {
	path := fmt.Sprintf("/tags/%s", url.PathEscape(tagID))

	if params != nil && params.IncludeTemplate != nil {
		urlParams := url.Values{}
		urlParams.Add("include_template", fmt.Sprintf("%t", *params.IncludeTemplate))
		path += "?" + urlParams.Encode()
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var tag Tag
	if err := json.Unmarshal(respBody, &tag); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &tag, nil
}

// GetTagBySlug 通过 slug 获取特定标签
// 参考: https://gamma-api.polymarket.com/tags/slug/{slug}
func (c *Client) GetTagBySlug(ctx context.Context, slug string, params *GetTagBySlugQueryParams) (*Tag, error) {
	path := fmt.Sprintf("/tags/slug/%s", url.PathEscape(slug))

	if params != nil && params.IncludeTemplate != nil {
		urlParams := url.Values{}
		urlParams.Add("include_template", fmt.Sprintf("%t", *params.IncludeTemplate))
		path += "?" + urlParams.Encode()
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var tag Tag
	if err := json.Unmarshal(respBody, &tag); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &tag, nil
}

// GetRelatedTagsByID 通过标签 ID 获取相关标签关系
// 参考: https://gamma-api.polymarket.com/tags/{id}/related-tags
func (c *Client) GetRelatedTagsByID(ctx context.Context, tagID string, params *GetRelatedTagsParams) ([]TagRelationship, error) {
	path := fmt.Sprintf("/tags/%s/related-tags", url.PathEscape(tagID))

	if params != nil {
		urlParams := url.Values{}
		if params.OmitEmpty != nil {
			urlParams.Add("omit_empty", fmt.Sprintf("%t", *params.OmitEmpty))
		}
		if params.Status != "" {
			urlParams.Add("status", string(params.Status))
		}
		if len(urlParams) > 0 {
			path += "?" + urlParams.Encode()
		}
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var relationships []TagRelationship
	if err := json.Unmarshal(respBody, &relationships); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return relationships, nil
}

// GetRelatedTagsBySlug 通过标签 slug 获取相关标签关系
// 参考: https://gamma-api.polymarket.com/tags/slug/{slug}/related-tags
func (c *Client) GetRelatedTagsBySlug(ctx context.Context, slug string, params *GetRelatedTagsParams) ([]TagRelationship, error) {
	path := fmt.Sprintf("/tags/slug/%s/related-tags", url.PathEscape(slug))

	if params != nil {
		urlParams := url.Values{}
		if params.OmitEmpty != nil {
			urlParams.Add("omit_empty", fmt.Sprintf("%t", *params.OmitEmpty))
		}
		if params.Status != "" {
			urlParams.Add("status", string(params.Status))
		}
		if len(urlParams) > 0 {
			path += "?" + urlParams.Encode()
		}
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var relationships []TagRelationship
	if err := json.Unmarshal(respBody, &relationships); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return relationships, nil
}

// GetRelatedTagsDetailByID 获取与给定标签 ID 相关的标签的详细信息
// 参考: https://gamma-api.polymarket.com/tags/{id}/related-tags/tags
func (c *Client) GetRelatedTagsDetailByID(ctx context.Context, tagID string, params *GetRelatedTagsParams) ([]Tag, error) {
	path := fmt.Sprintf("/tags/%s/related-tags/tags", url.PathEscape(tagID))

	if params != nil {
		urlParams := url.Values{}
		if params.OmitEmpty != nil {
			urlParams.Add("omit_empty", fmt.Sprintf("%t", *params.OmitEmpty))
		}
		if params.Status != "" {
			urlParams.Add("status", string(params.Status))
		}
		if len(urlParams) > 0 {
			path += "?" + urlParams.Encode()
		}
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var tags []Tag
	if err := json.Unmarshal(respBody, &tags); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return tags, nil
}

// GetRelatedTagsDetailBySlug 获取与给定标签 slug 相关的标签的详细信息
// 参考: https://gamma-api.polymarket.com/tags/slug/{slug}/related-tags/tags
func (c *Client) GetRelatedTagsDetailBySlug(ctx context.Context, slug string, params *GetRelatedTagsParams) ([]Tag, error) {
	path := fmt.Sprintf("/tags/slug/%s/related-tags/tags", url.PathEscape(slug))

	if params != nil {
		urlParams := url.Values{}
		if params.OmitEmpty != nil {
			urlParams.Add("omit_empty", fmt.Sprintf("%t", *params.OmitEmpty))
		}
		if params.Status != "" {
			urlParams.Add("status", string(params.Status))
		}
		if len(urlParams) > 0 {
			path += "?" + urlParams.Encode()
		}
	}

	respBody, err := c.doRequest(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var tags []Tag
	if err := json.Unmarshal(respBody, &tags); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return tags, nil
}

