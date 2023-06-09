/*
 * Jacobin VM - A Java virtual machine
 * Copyright (c) 2023 by Andrew Binstock. All rights reserved.
 * Licensed under Mozilla Public License 2.0 (MPL 2.0)
 */

package classloader

// Statics is a fast-lookup map of static variables and functions. The int64 value
// contains the index into the statics array where the entry is stored.
// Statics are placed into this map only when they are first referenced and resolved.
var Statics = make(map[string]int64)
var StaticsArray []Static

// Static contains all the various items needed for a static variable or function.
type Static struct {
    Class byte // the kind of entity we're dealing with
    /*
    	B	byte signed byte
    	C	char	Unicode character code point (UTF-16)
    	D	double
    	F	float
    	I	int	integer
    	J	long integer
    	L ClassName ;	reference	an instance of class ClassName
    	S	signed short int
    	Z	boolean
    	plus (Jacobin implementation-specific):
    	G   native method (that is, one written in Go
    */
    Type      string  // Type data used for reference variables (i.e., objects, etc.)
    ValueRef  string  // pointer--might need to change this
    ValueInt  int64   // holds longs, ints, shorts, chars, booleans, byte
    ValueFP   float64 // holds doubles and floats
    ValueStr  string  // string
    ValueFunc func()  // function pointer
    CP        *CPool  // the constant pool for the class
}
