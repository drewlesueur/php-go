package zend

/*
#include "extension.h"

#include <php.h>
#include <zend_exceptions.h>
#include <zend_interfaces.h>
#include <zend_ini.h>
#include <SAPI.h>

*/
import "C"
import "unsafe"
import "reflect"
import "errors"
import "fmt"

var (
	ExtName string = ""
	ExtVer  string = "1.0"
)

func InitExtension(name string, version ...string) {
	ExtName = name
	if len(version) > 0 {
		ExtVer = version[0]
	}
}

var gext = NewExtension()

type FuncEntry struct {
	class  string
	method string
	fn     interface{}
}

func NewFuncEntry(class string, method string, fn interface{}) *FuncEntry {
	return &FuncEntry{class, method, fn}
}

func (this *FuncEntry) Name() string {
	return this.class + "_" + this.method
}

func (this *FuncEntry) IsGlobal() bool {
	return this.class == "global"
}

type Extension struct {
	syms    map[string]int
	classes map[string]int
	cbs     map[int]*FuncEntry // cbid => callable callbak

	fidx int // = 0

	me *C.zend_module_entry
	fe *C.zend_function_entry
}

// TODO 把entry位置与cbid分开，这样cbfunc就能够更紧凑了
func NewExtension() *Extension {
	syms := make(map[string]int, 0)
	classes := make(map[string]int, 0)
	cbs := make(map[int]*FuncEntry, 0)

	classes["global"] = 0 // 可以看作内置函数的类
	return &Extension{syms: syms, classes: classes, cbs: cbs}
}

// depcreated
func gencbid(cidx int, fidx int) int {
	return cidx*128 + fidx
}

func nxtcbid() int {
	return len(gext.syms)
}

func AddFunc(name string, f interface{}) error {
	fe := NewFuncEntry("global", name, f)
	sname := fe.Name()

	if _, has := gext.syms[sname]; !has {
		// TODO check f type

		cidx := 0
		fidx := gext.fidx
		// cbid := gencbid(0, fidx)
		cbid := nxtcbid()

		cname := C.CString(name)
		n := C.zend_add_function(C.int(cidx), C.int(fidx), C.int(cbid), cname)
		C.free(unsafe.Pointer(cname))

		if int(n) == 0 {
			gext.syms[sname] = cbid
			gext.cbs[cbid] = fe
			gext.fidx += 1
			return nil
		}
	}

	return errors.New("add func error.")
}

// 添加新类的时候，可以把类的公共方法全部导出吧
// 不用逐个方法添加，简单多了。
func AddClass(name string, ctor interface{}) error {

	if _, has := gext.classes[name]; !has {
		cidx := len(gext.classes)

		cname := C.CString(name)
		n := C.zend_add_class(C.int(cidx), cname)
		C.free(unsafe.Pointer(cname))

		if int(n) == 0 {
			gext.classes[name] = cidx

			addCtor(name, ctor)
			addMethods(name, ctor)
			return nil
		}
	}

	return errors.New("add class error.")
}

func addCtor(cname string, ctor interface{}) {
	mname := "__construct"
	fidx := 0
	addMethod(fidx, cname, mname, ctor)
}

func addMethods(cname string, ctor interface{}) {
	fty := reflect.TypeOf(ctor)
	cls := fty.Out(0)

	for i := 0; i < cls.NumMethod(); i++ {
		fmt.Println(i, cname, cls.Method(i).Name)
		addMethod(i+1, cname, cls.Method(i).Name, nil)
	}
}

func addMethod(fidx int, cname string, mname string, fn interface{}) {
	cidx := gext.classes[cname]
	// cbid := gencbid(cidx, fidx)
	cbid := nxtcbid()

	fe := NewFuncEntry(cname, mname, fn)

	ccname := C.CString(cname)
	cmname := C.CString(mname)

	mn := C.zend_add_method(C.int(cidx), C.int(fidx), C.int(cbid), ccname, cmname)
	C.free(unsafe.Pointer(ccname))
	C.free(unsafe.Pointer(cmname))

	if mn == 0 {
		gext.cbs[cbid] = fe
		gext.syms[fe.Name()] = cbid
	}
}

// 内置函数注册，内置类注册。
func addBuiltins() {
	// nice fix exit crash bug.
	// AddFunc("GoExit", func() { os.Exit(0) })
}

//export on_phpgo_function_callback
func on_phpgo_function_callback(no int) {
	fmt.Println("go callback called:", no, gext.cbs[no])
	fe := gext.cbs[no]
	fe.fn.(func())()
}
