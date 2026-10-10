package dynamo

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
)

// errInvalidMapKey guards a server-generated id that mapKey refuses - never
// expected, since ids are UUIDs.
var errInvalidMapKey = errors.New("id can't be used as a map key")

// mapKey is the path to key within the map attribute attr (images, kits,
// plates, pcbs). key is often a request's path value, so it's kept as one
// name, never split on '.' - otherwise "abc.material" would address a field
// inside entry abc. ok is false when key holds '[' or ']', which the
// expression builder reads as a list index even so; no stored key does.
func mapKey(attr, key string) (path expression.NameBuilder, ok bool) {
	if key == "" || strings.ContainsAny(key, "[]") {
		return expression.NameBuilder{}, false
	}
	return expression.Name(attr).AppendName(expression.NameNoDotSplit(key)), true
}

// field is the path to name within a map entry from mapKey.
func field(entry expression.NameBuilder, name string) expression.NameBuilder {
	return entry.AppendName(expression.Name(name))
}

// setOrRemoveName is setOrRemovePtr for a path built with mapKey/field.
func setOrRemoveName[T any](update expression.UpdateBuilder, name expression.NameBuilder, v *T) expression.UpdateBuilder {
	if v == nil {
		return update.Remove(name)
	}
	return update.Set(name, expression.Value(*v))
}
