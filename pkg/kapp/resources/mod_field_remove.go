// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"fmt"
	"regexp"
)

type FieldRemoveMod struct {
	ResourceMatcher ResourceMatcher
	Path            Path
}

var _ ResourceMod = FieldRemoveMod{}
var _ ResourceModWithMultiple = FieldCopyMod{}

func (t FieldRemoveMod) ApplyFromMultiple(res Resource, _ map[FieldCopyModSource]Resource) error {
	return t.Apply(res)
}

func (t FieldRemoveMod) Apply(res Resource) error {
	if !t.ResourceMatcher.Matches(res) {
		return nil
	}
	err := t.apply(res.unstructured().Object, t.Path)
	if err != nil {
		return fmt.Errorf("FieldRemoveMod for path '%s' on resource '%s': %w", t.Path.AsString(), res.Description(), err)
	}
	return nil
}

func (t FieldRemoveMod) apply(obj interface{}, path Path) error {
	for i, part := range path {
		isLast := len(path) == i+1

		switch {
		case part.MapKey != nil:
			typedObj, ok := obj.(map[string]interface{})
			if !ok {
				// TODO check strictness?
				if typedObj == nil {
					return nil // map is a nil, nothing to remove
				}
				return fmt.Errorf("Unexpected non-map found: %T", obj)
			}

			if isLast {
				delete(typedObj, *part.MapKey)
				return nil
			}

			var found bool
			obj, found = typedObj[*part.MapKey]
			if !found {
				return nil // map key is not found, nothing to remove
			}

		case part.ArrayIndex != nil:
			if isLast && part.RegexObj.Regex == nil {
				return fmt.Errorf("Expected last part of the path to be map key")
			}

			switch {
			case part.ArrayIndex.All != nil:
				typedObj, ok := obj.([]interface{})
				if !ok {
					return fmt.Errorf("Unexpected non-array found: %T", obj)
				}

				for _, obj := range typedObj {
					err := t.apply(obj, path[i+1:])
					if err != nil {
						return err
					}
				}
				return nil // dealt with children, get out

			case part.ArrayIndex.Index != nil:
				typedObj, ok := obj.([]interface{})
				if !ok {
					return fmt.Errorf("Unexpected non-array found: %T", obj)
				}

				if *part.ArrayIndex.Index < len(typedObj) {
					obj = typedObj[*part.ArrayIndex.Index]
				} else {
					return nil // index not found, nothing to remove
				}

			default:
				panic(fmt.Sprintf("Unknown array index: %#v", part.ArrayIndex))
			}
		case part.RegexObj.Regex != nil:
			matchedKeys, err := t.obtainMatchingRegexKeys(obj, part)
			if err != nil {
				return err
			}
			for _, key := range matchedKeys {
				newPath := append(Path{&PathPart{MapKey: &key}}, path[i+1:]...)
				err := t.apply(obj, newPath)
				if err != nil {
					return err
				}
			}
			return nil

		default:
			panic(fmt.Sprintf("Unexpected path part: %#v", part))
		}
	}

	panic("unreachable")
}

func (t FieldRemoveMod) obtainMatchingRegexKeys(obj interface{}, part *PathPart) ([]string, error) {
	var matchedKeys []string
	regex, err := regexp.Compile(*part.RegexObj.Regex)
	if err != nil {
		return matchedKeys, err
	}
	typedObj, ok := obj.(map[string]interface{})
	if !ok && typedObj != nil {
		return matchedKeys, fmt.Errorf("Unexpected non-map found: %T", obj)
	}
	for key := range typedObj {
		if regex.MatchString(key) {
			matchedKeys = append(matchedKeys, key)
		}
	}
	return matchedKeys, nil
}
