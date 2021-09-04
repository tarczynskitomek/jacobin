/*
 * Jacobin VM - A Java virtual machine
 * Copyright (c) 2021 by Andrew Binstock. All rights reserved.
 * Licensed under Mozilla Public License 2.0 (MPL 2.0)
 */

package classloader

import (
	"fmt"
	"jacobin/globals"
	"jacobin/log"
	"os"
	"strconv"
)

// reads in a class file, parses it, and puts the values into the fields of the
// class that will be loaded into the classloader. Some verification performed
// receives the rawBytes of the class that were previously read in
//
// ClassFormatError - if the parser finds anything unexpected
func parse(rawBytes []byte) (parsedClass, error) {

	// the parsed class as we'll give it to the classloader
	var pClass = parsedClass{}

	err := parseMagicNumber(rawBytes)
	if err != nil {
		return pClass, err
	}

	err = parseJavaVersionNumber(rawBytes, &pClass)
	if err != nil {
		return pClass, err
	}

	err = getConstantPoolCount(rawBytes, &pClass)
	if err != nil {
		return pClass, err
	}

	pos, err := parseConstantPool(rawBytes, &pClass)
	if err != nil || pos < 10 {
		return pClass, err
	}

	pos, err = parseAccessFlags(rawBytes, pos, &pClass)
	if err != nil {
		return pClass, err
	}

	pos, err = parseClassName(rawBytes, pos, &pClass)
	if err != nil {
		return pClass, err
	}

	pos, err = parseSuperClassName(rawBytes, pos, &pClass)
	if err != nil {
		return pClass, err
	}

	return pClass, nil
}

// all bytecode files start with 0xCAFEBABE ( it was the 90s!)
// this checks for that.
func parseMagicNumber(bytes []byte) error {
	if len(bytes) < 4 {
		return cfe("invalid magic number")
	} else if (bytes[0] != 0xCA) || (bytes[1] != 0xFE) || (bytes[2] != 0xBA) || (bytes[3] != 0xBE) {
		return cfe("invalid magic number")
	} else {
		return nil
	}
}

// get the Java version number used in creating this class file. If it's higher than the
// version Jacobin presently supports, report an error.
func parseJavaVersionNumber(bytes []byte, klass *parsedClass) error {
	version, err := intFrom2Bytes(bytes, 6)
	if err != nil {
		return err
	}

	if version > globals.GetInstance().MaxJavaVersionRaw {
		errMsg := "Jacobin supports only Java versions through Java " +
			strconv.Itoa(globals.GetInstance().MaxJavaVersion)
		return cfe(errMsg)
	}

	klass.javaVersion = version
	log.Log("Java version: "+strconv.Itoa(version), log.FINEST)
	return nil
}

// get the number of entries in the constant pool. This number will
// be used later on to verify that the number of entries we fetch is
// correct. Note that this number is technically 1 greater than the
// number of actual entries, because the first entry in the constant
// pool is an empty placeholder, rather than an actual entry.
func getConstantPoolCount(bytes []byte, klass *parsedClass) error {
	cpEntryCount, err := intFrom2Bytes(bytes, 8)
	if err != nil || cpEntryCount <= 2 {
		return cfe("Invalid number of entries in constant pool: " +
			strconv.Itoa(cpEntryCount))
	} else {
		klass.cpCount = cpEntryCount
		log.Log("Number of CP entries: "+strconv.Itoa(cpEntryCount), log.FINEST)
		return nil
	}
}

// decode the meaning of the class access flags and set the various getters
// in the class. FromTable 4.1-B in the spec:
// https://docs.oracle.com/javase/specs/jvms/se11/html/jvms-4.html#jvms-4.1-200-E.1
func parseAccessFlags(bytes []byte, loc int, klass *parsedClass) (int, error) {
	pos := loc
	accessFlags, err := intFrom2Bytes(bytes, pos+1)
	pos += 2
	if err != nil {
		return pos, cfe("Invalid get of class access flags")
	} else {
		klass.accessFlags = accessFlags
		if accessFlags&0x0001 > 0 {
			klass.classIsPublic = true
		}
		if accessFlags&0x0010 > 0 {
			klass.classIsFinal = true
		}
		if accessFlags&0x0020 > 0 {
			klass.classIsSuper = true
		}
		if accessFlags&0x0200 > 0 {
			klass.classIsInterface = true
		}
		if accessFlags&0x0400 > 0 {
			klass.classIsAbstract = true
		}
		if accessFlags&0x1000 > 0 {
			klass.classIsSynthetic = true
		} // is generated by the JVM, is not in the program
		if accessFlags&0x2000 > 0 {
			klass.classIsAnnotation = true
		}
		if accessFlags&0x4000 > 0 {
			klass.classIsEnum = true
		}
		if accessFlags&0x8000 > 0 {
			klass.classIsModule = true
		}
		log.Log("Access flags: "+strconv.Itoa(accessFlags), log.FINEST)

		if log.LogLevel == log.FINEST {
			if klass.classIsPublic {
				fmt.Fprintf(os.Stderr, "access: public\n")
			}
			if klass.classIsFinal {
				fmt.Fprintf(os.Stderr, "access: final\n")
			}
			if klass.classIsSuper {
				fmt.Fprintf(os.Stderr, "access: super\n")
			}
			if klass.classIsInterface {
				fmt.Fprintf(os.Stderr, "access: interface\n")
			}
			if klass.classIsAbstract {
				fmt.Fprintf(os.Stderr, "access: abstract\n")
			}
			if klass.classIsSynthetic {
				fmt.Fprintf(os.Stderr, "access: synthetic\n")
			}
			if klass.classIsAnnotation {
				fmt.Fprintf(os.Stderr, "access: annotation\n")
			}
			if klass.classIsEnum {
				fmt.Fprintf(os.Stderr, "access: enum\n")
			}
			if klass.classIsModule {
				fmt.Fprintf(os.Stderr, "access: module\n")
			}
		}
		return pos, nil
	}
}

// The value for this item points to a CP entry of type Class info. In turn,
// that entry points to the UTF-8 name of the class. This name includes the
// package name as a path, but not the extension of .class. So for example,
// ParsePosition.class in the core Java string library has a class name of:
// java/text/ParsePosition
func parseClassName(bytes []byte, loc int, klass *parsedClass) (int, error) {
	pos := loc
	index, err := intFrom2Bytes(bytes, pos+1)
	var classNameIndex int
	pos += 2
	if err != nil {
		return pos, cfe("error obtaining index for class name")
	}

	if index < 1 || index > (len(klass.cpIndex)-1) {
		return pos, cfe("invalid index into CP for class name")
	}

	pointedToClassRef := klass.cpIndex[index]
	if pointedToClassRef.entryType != ClassRef {
		return pos, cfe("invalid entry for class name")
	}

	// the entry pointed to by pointedToClassRef holds an index to
	// a UTF-8 string that holds the class name
	classNameIndex = klass.classRefs[pointedToClassRef.slot].index
	if klass.cpIndex[classNameIndex].entryType != UTF8 {
		return pos, cfe("error classRef in CP does not point to a UTF-8 string")
	}

	// get the slot # in the UTF-8 slice for this name string, then retrieve it.
	utf8Index := klass.cpIndex[classNameIndex].slot
	className := klass.utf8Refs[utf8Index].content
	log.Log("class name: "+className, log.FINEST)
	klass.className = className
	return pos, nil
}

// Get the name of the superclass. The logic is identical to that of parseClassName()
// All classes, except java/lang/Object have superclasses.
func parseSuperClassName(bytes []byte, loc int, klass *parsedClass) (int, error) {
	pos := loc
	index, err := intFrom2Bytes(bytes, pos+1)
	var classNameIndex int
	pos += 2
	if err != nil {
		return pos, cfe("error obtaining index for superclass name")
	}

	if index < 1 || index > (len(klass.cpIndex)-1) {
		return pos, cfe("invalid index into CP for superclass name")
	}

	pointedToClassRef := klass.cpIndex[index]
	if pointedToClassRef.entryType != ClassRef {
		return pos, cfe("invalid entry for superclass name")
	}

	// the entry pointed to by pointedToClassRef holds an index to
	// a UTF-8 string that holds the class name
	classNameIndex = klass.classRefs[pointedToClassRef.slot].index
	if klass.cpIndex[classNameIndex].entryType != UTF8 {
		return pos, cfe("error classRef in CP does not point to a UTF-8 string")
	}

	// get the slot # in the UTF-8 slice for this name string, then retrieve it.
	utf8Index := klass.cpIndex[classNameIndex].slot
	superClassName := klass.utf8Refs[utf8Index].content

	if superClassName == "" && klass.className != "java/lang/Object" {
		return pos, cfe("Invaild empty string for superclass name")
	}

	log.Log("superclass name: "+superClassName, log.FINEST)
	klass.superClass = superClassName
	return pos, nil
}
