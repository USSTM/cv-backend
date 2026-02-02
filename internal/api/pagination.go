package api

// parsePagination normalizes limit/offset query params.
// limit=50, offset=0. limit capped at 100, minimum 1.
// offset min 0
func parsePagination(limit, offset *int) (int64, int64) {
	l := int64(50)
	o := int64(0)
	if limit != nil {
		l = int64(*limit)
	}
	if offset != nil {
		o = int64(*offset)
	}
	if l > 100 {
		l = 100
	}
	if l < 1 {
		l = 1
	}
	if o < 0 {
		o = 0
	}
	return l, o
}
