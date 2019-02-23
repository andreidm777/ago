package guava

const key int64 = 2862933555777941757

const dou float64 = 2147483648

func Guava(state int64, buckets int32) int32 {
	var next_double float64
	var candidate int32
	candidate = 0
	var next int32
	for 1 == 1 {
		state = key*state + 1
		next_double = float64(int32(uint64(state)>>33)+1) / dou
		next = int32(float64(candidate+1) / next_double)

		if next >= 0 && next < buckets {
			candidate = next
		} else {
			return candidate
		}
	}
	return candidate
}
