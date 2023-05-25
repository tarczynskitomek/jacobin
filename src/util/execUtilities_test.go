/*
 * Jacobin VM - A Java virtual machine
 * Copyright (c) 2022-3 by the Jacobin authors. All rights reserved.
 * Licensed under Mozilla Public License 2.0 (MPL 2.0)
 */

package util

import (
    "testing"
)

// verify that a trailing slash in JAVA_HOME is removed
func TestParseIncomingParamsFromMethType(t *testing.T) {
    res := ParseIncomingParamsFromMethTypeString("(SBI)")
    if len(res) != 3 { // short, byte and int all become 'I'
        t.Errorf("Expected 3 parsed parameters, got %d", len(res))
    }

    if res[0] != "I" || res[1] != "I" || res[2] != "I" {
        t.Errorf("Expected parse would return 3 values of 'I', got: %s%s%s",
            res[0], res[1], res[2])
    }

    res = ParseIncomingParamsFromMethTypeString("(S[BI)I")
    if len(res) != 3 { // short, byte and int all become 'I'
        t.Errorf("Expected 3 parsed parameters, got %d", len(res))
    }

    if res[0] != "I" || res[1] != "[B" || res[2] != "I" {
        t.Errorf("Expected parse would return S [B I, got: %s %s %s",
            res[0], res[1], res[2])
    }

    res = ParseIncomingParamsFromMethTypeString("")
    if len(res) != 0 {
        t.Errorf("Expected parse would return value an empty string array, got: %s", res)
    }
}
