package api

import v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"

// pageToken extracts the page token from a PageRequest, returning "" if nil.
func pageToken(p *v1.PageRequest) string {
	if p == nil {
		return ""
	}
	return p.PageToken
}

// pageSize extracts the page size from a PageRequest, returning 0 if nil.
func pageSize(p *v1.PageRequest) uint32 {
	if p == nil {
		return 0
	}
	return p.PageSize
}
