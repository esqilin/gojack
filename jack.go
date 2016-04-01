package gojack

// #include <stdlib.h>
// #include <jack/jack.h>
import "C"
import (
	"reflect"
	"unsafe"
)

func convCStrArr(cArr **C.char) []*C.char {
	n := unsafe.Sizeof(cArr) / C.sizeof_char
	arr := (*[1 << 30]*C.char)(unsafe.Pointer(cArr))[:n:n]
	C.free(unsafe.Pointer(cArr))
	return arr
}

func convCFloat32Arr(cArr *C.float, n int, arr *[]float32) {
	h := (*reflect.SliceHeader)((unsafe.Pointer(arr)))
	h.Cap = n
	h.Len = n
	h.Data = uintptr(unsafe.Pointer(cArr))
}
