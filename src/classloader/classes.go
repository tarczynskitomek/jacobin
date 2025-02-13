/*
 * Jacobin VM - A Java virtual machine
 * Copyright (c) 2021-2 by the Jacobin authors. All rights reserved.
 * Licensed under Mozilla Public License 2.0 (MPL 2.0)
 */

package classloader

import (
	"errors"
	"fmt"
	"jacobin/log"
	"jacobin/shutdown"
)

// the definition of the class as it's stored in the method area
type Klass struct {
	Status byte // I=Initializing,F=formatChecked,V=verified,L=linked,N=instantiated
	Loader string
	Data   *ClData
}

type ClData struct {
	Name        string
	Superclass  string
	Module      string
	Pkg         string   // package name, if any. (so named, b/c 'package' is a golang keyword)
	Interfaces  []uint16 // indices into UTF8Refs
	Fields      []Field
	MethodTable map[string]*Method
	Methods     []Method
	Attributes  []Attr
	SourceFile  string
	Bootstraps  []BootstrapMethod
	CP          CPool
	Access      AccessFlags
	ClInit      byte // 0 = no clinit, 1 = clinit not run, 2 clinit run
}

type CPool struct {
	CpIndex        []CpEntry // the constant pool index to entries
	ClassRefs      []uint16  // points to a UTF8 entry in the CP bearing class name
	Doubles        []float64
	Dynamics       []DynamicEntry
	FieldRefs      []FieldRefEntry
	Floats         []float32
	IntConsts      []int32 // 32-bit int containing the actual int value
	InterfaceRefs  []InterfaceRefEntry
	InvokeDynamics []InvokeDynamicEntry
	LongConsts     []int64
	MethodHandles  []MethodHandleEntry
	MethodRefs     []MethodRefEntry
	MethodTypes    []uint16
	NameAndTypes   []NameAndTypeEntry
	//	StringRefs     []uint16 // all StringRefs are converted into utf8Refs
	Utf8Refs []string
}

type AccessFlags struct {
	ClassIsPublic     bool
	ClassIsFinal      bool
	ClassIsSuper      bool
	ClassIsInterface  bool
	ClassIsAbstract   bool
	ClassIsSynthetic  bool
	ClassIsAnnotation bool
	ClassIsEnum       bool
	ClassIsModule     bool
}

// For the nonce, these definitions are similar to corresponding items in
// classloader.go. The biggest difference is that ints there often become uint16
// here (where correct to do so). This greatly reduces memory consumption.
// Likewise certain fields needed there (counts) are not used here.

type Field struct {
	AccessFlags int
	Name        uint16 // index of the UTF-8 entry in the CP
	Desc        uint16 // index of the UTF-8 entry in the CP
	IsStatic    bool   // is the field static?
	Attributes  []Attr
}

// the methods of the class, including the constructors
type Method struct {
	AccessFlags int
	Name        uint16 // index of the UTF-8 entry in the CP
	Desc        uint16 // index of the UTF-8 entry in the CP
	CodeAttr    CodeAttrib
	Attributes  []Attr
	Exceptions  []uint16 // indexes into Utf8Refs in the CP
	Parameters  []ParamAttrib
	Deprecated  bool // is the method deprecated?
}

type CodeAttrib struct {
	MaxStack   int
	MaxLocals  int
	Code       []byte
	Exceptions []CodeException // exception entries for this method
	Attributes []Attr          // the code attributes has its own sub-attributes(!)
}

// ParamAttrib is the MethodParameters method attribute
type ParamAttrib struct {
	Name        string // string, rather than index into utf8Refs b/c the name could be ""
	AccessFlags int
}

// the structure of many attributes (field, class, etc.) The content is just the raw bytes.
type Attr struct {
	AttrName    uint16 // index of the UTF8 entry in the CP
	AttrSize    int    // length of the following array of raw bytes
	AttrContent []byte // the raw data of the attribute
}

// the exception-related data for each exception in the Code attribute of a given method
type CodeException struct {
	StartPc   int    // first instruction covered by this exception (pc = program counter)
	EndPc     int    // the last instruction covered by this exception
	HandlerPc int    // the place in the method code that has the exception instructions
	CatchType uint16 // the type of exception, index to CP, which must point a ClassFref entry
}

// the bootstrap methods, specified in the bootstrap class attribute
type BootstrapMethod struct {
	MethodRef uint16   // index pointing to a MethodHandle
	Args      []uint16 // arguments: indexes to loadable arguments from the CP
}

// ==== Constant Pool structs (in order by their numeric code) ====//
type CpEntry struct {
	Type uint16
	Slot uint16
}

type FieldRefEntry struct { // type: 09 (field reference)
	ClassIndex  uint16
	NameAndType uint16
}

type MethodRefEntry struct { // type: 10 (method reference)
	ClassIndex  uint16
	NameAndType uint16
}

type InterfaceRefEntry struct { // type: 11 (interface reference)
	ClassIndex  uint16
	NameAndType uint16
}

type NameAndTypeEntry struct { // type 12 (name and type reference)
	NameIndex uint16
	DescIndex uint16
}

type MethodHandleEntry struct { // type: 15 (method handle)
	RefKind  uint16
	RefIndex uint16
}

type DynamicEntry struct { // type 17 (dynamic--similar to invokedynamic)
	BootstrapIndex uint16
	NameAndType    uint16
}

type InvokeDynamicEntry struct { // type 18 (invokedynamic data)
	BootstrapIndex uint16
	NameAndType    uint16
}

// // the various types of entries in the constant pool. These entries are duplicates
// // of the ones in cpParser.go. These lists should be kept in sync.
// const (
// 	Dummy              = 0 // used for initialization and for dummy entries (viz. for longs, doubles)
// 	UTF8               = 1
// 	IntConst           = 3
// 	FloatConst         = 4
// 	LongConst          = 5
// 	DoubleConst        = 6
// 	ClassRef           = 7
// 	StringConst        = 8
// 	FieldRef           = 9
// 	MethodRef          = 10
// 	Interface          = 11
// 	NameAndType        = 12
// 	MethodHandle       = 15
// 	MethodType         = 16
// 	DynamicEntry       = 17
// 	InvokeDynamicEntry = 18
// 	Module             = 19
// 	Package            = 20
// )

// FetchMethodAndCP gets a method and the CP for the class of the method. It searches
// for the method first by checking the global MTable (that is, the global method table).
// If it doesn't find it there, then it looks for the method in the class entry in MethArea.
// If it finds it there, then it loads that class into the MTable and returns that
// entry as the Method it's returning.
//
// Note that if the given method is not found, the hierarchy of superclasses is ascended,
// in search for the method. The one exception is for main() which, if not found in the
// first class, will never be in one of the superclasses.
func FetchMethodAndCP(className, methName, methType string) (MTentry, error) {
	origClassName := className
	// for {
	// startSearch:

	// has the className been loaded? If not, then load it now.
	if MethAreaFetch(className) == nil {
		err := LoadClassFromNameOnly(className)
		if err != nil {
			if methName == "main" {
				// the starting className is always loaded, so if main() isn't found
				// right away, just bail.
				noMainError(origClassName)
				shutdown.Exit(shutdown.JVM_EXCEPTION)
			}
			_ = log.Log("FetchMethodAndCP: LoadClassFromNameOnly for "+className+" failed: "+err.Error(), log.WARNING)
			_ = log.Log(err.Error(), log.SEVERE)
			shutdown.Exit(shutdown.JVM_EXCEPTION)
		}
	}

	methFQN := className + "." + methName + methType // FQN = fully qualified name
	methEntry := MTable[methFQN]

	if methEntry.Meth != nil { // we found the entry in the MTable
		if methEntry.MType == 'J' {
			return MTentry{Meth: methEntry.Meth, MType: 'J'}, nil
		} else if methEntry.MType == 'G' {
			return MTentry{Meth: methEntry.Meth, MType: 'G'}, nil
		}
	}

	// method is not in the MTable, so find it and put it there
	err := WaitForClassStatus(className)
	if err != nil {
		errMsg := fmt.Sprintf("FetchMethodAndCP: %s", err.Error())
		_ = log.Log(errMsg, log.SEVERE)
		shutdown.Exit(shutdown.JVM_EXCEPTION)
		return MTentry{}, errors.New(errMsg) // dummy return needed for tests
	}

	k := MethAreaFetch(className)
	if k == nil {
		errMsg := fmt.Sprintf("FetchMethodAndCP: MethAreaFetch could not find class %s", className)
		_ = log.Log(errMsg, log.SEVERE)
		shutdown.Exit(shutdown.JVM_EXCEPTION)
		return MTentry{}, errors.New(errMsg) // dummy return needed for tests
	}

	if k.Loader == "" { // if className is not found, the zero value struct is returned
		// TODO: check superclasses if method not found
		errMsg := "FetchMethodAndCP: Null Loader in className: " + className
		_ = log.Log(errMsg, log.SEVERE)
		return MTentry{}, errors.New(errMsg) // dummy return needed for tests
	}

	// the className has been found (k) so check the method table. Then return the
	// method along with a pointer to the CP
	var m Method
	searchName := methName + methType
	methRef, ok := k.Data.MethodTable[searchName]
	if ok {
		m = *methRef

		// create a Java method struct for this method. We know it's a Java method
		// because if it were a native method it would have been found in the initial
		// lookup in the MTable (as all native methods are loaded there before
		// program execution begins.
		jme := JmEntry{
			AccessFlags: m.AccessFlags,
			MaxStack:    m.CodeAttr.MaxStack,
			MaxLocals:   m.CodeAttr.MaxLocals,
			Code:        m.CodeAttr.Code,
			Exceptions:  m.CodeAttr.Exceptions,
			attribs:     m.CodeAttr.Attributes,
			params:      m.Parameters,
			deprecated:  m.Deprecated,
			Cp:          &k.Data.CP,
		}
		MTable[methFQN] = MTentry{
			Meth:  jme,
			MType: 'J',
		}
		return MTentry{Meth: jme, MType: 'J'}, nil
	}

	// if we're here, the className did not contain the searched-for method. So, go up the superclasses,
	// except if we're searching for main(), in which case, we don't go up the list of superclasses
	if methName == "main" { // to be consistent with the JDK, we print this peculiar error message when main() is missing
		noMainError(origClassName)
		// break
	}

	// if className == "java/lang/Object" { // if we're already at the topmost superclass, then stop the loop
	// 	break
	// } else {
	// 	className = k.Data.Superclass
	// 	goto startSearch
	// }
	// }

	// if we got this far, something went wrong with locating the method
	msg := "FetchMethodAndCP: Found class " + className + ", but it did not contain method: " + methName
	return MTentry{}, errors.New(msg)
}

// error message when main() can't be found
func noMainError(className string) {
	_ = log.Log("Error: main() method not found in class "+className+"\n"+
		"Please define the main method as:\n"+
		"   public static void main(String[] args)", log.SEVERE)
}

// FetchUTF8stringFromCPEntryNumber fetches the UTF8 string using the CP entry number
// for that string in the designated ClData.CP. Returns "" on error.
func FetchUTF8stringFromCPEntryNumber(cp *CPool, entry uint16) string {
	if entry < 1 || entry >= uint16(len(cp.CpIndex)) {
		msg := fmt.Sprintf("FetchUTF8stringFromCPEntryNumber: entry=%d is out of bounds(1, %d)", entry, uint16(len(cp.CpIndex)))
		_ = log.Log(msg, log.SEVERE)
		return ""
	}

	u := cp.CpIndex[entry]
	if u.Type != UTF8 {
		msg := fmt.Sprintf("FetchUTF8stringFromCPEntryNumber: cp.CpIndex[%d].Type=%d, expected UTF8", entry, u.Type)
		_ = log.Log(msg, log.SEVERE)
		return ""
	}

	return cp.Utf8Refs[u.Slot]
}
