/*
 * Jacobin VM - A Java virtual machine
 * Copyright (c) 2021 by Andrew Binstock. All rights reserved.
 * Licensed under Mozilla Public License 2.0 (MPL 2.0)
 */

package lib

import (
	"fmt"
	"os"
)

/*
 Each object or library that has Go methods contains a reference to MethodSignatures,
 which contain data needed to insert the go method into the vtable of the currently
 executing JVM. MethodSignatures is a map whose key is the fully qualified name and
 type of the method (that is, the method's full signature) and a value consisting of
 a struct of an int (the number of slots to pop off the caller's operand stack when
 creating the new frame and a function. All methods have the same signature, regardless
 of the signature of their Java counterparts. That signature is that it accepts a slice
 of interface{} and returns nothing.

 The slice contains one entry for every parameter passed to the method (which could
 mean an empty slice). There is no return value, because the method will place any
 return value on the operand stack of the calling function.
*/

var MethodSignatures = make(map[string]method)

type method struct {
	paramSlots int
	fu         function
}

type function func([]interface{})

func load() {
	MethodSignatures["println"] = method{
		paramSlots: 2, // [0] = out object, [1] = string to print
		fu:         Println,
	}
}

// a temporary stand-in for java\io\PrintStream
type stream *os.File

var Out stream

func PrintStream(out stream) {
	Out = out
}

func init() {
	Out = os.Stdout
}

func Println(i []interface{}) {
	fmt.Fprintln(os.Stderr, i[0])
}
