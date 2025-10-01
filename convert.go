package fit

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/mitchellh/mapstructure"
)

// H container for map of strings to interface{}
type H map[string]any

// ToString map to string
func (h H) ToString() string {
	str, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(str)
}

func (h H) Value(key string) interface{} {
	if h == nil {
		return nil
	}

	return h[key]
}

func StructConvertMapByTag(obj interface{}, tagName string) map[string]interface{} {
	o := reflect.TypeOf(obj)
	v := reflect.ValueOf(obj)

	var data = make(map[string]interface{})
	for i := 0; i < o.NumField(); i++ {
		nt := o.Field(i).Tag.Get(tagName)
		if nt != "" && nt != "-" {
			data[nt] = v.Field(i).Interface()
		}
	}
	return data
}

func StructConvertSlice(obj interface{}, tagName string) []interface{} {
	mspRes := StructConvertMapByTag(obj, tagName)
	str := make([]interface{}, 0)
	for k, v := range mspRes {
		str = append(str, k)
		str = append(str, v)
	}
	return str
}

func MapConvertStruct(input, output interface{}) error {
	err := mapstructure.Decode(input, &output)
	if err != nil {
		log.Fatalln(err)
	}
	return nil
}

func MapConvertSlice(input interface{}) []interface{} {
	t := reflect.TypeOf(input).Kind()
	if t != reflect.Map {
		return nil
	}
	v := reflect.ValueOf(input)

	sli := make([]interface{}, 0)
	for _, k := range v.MapKeys() {
		sli = append(sli, k.Interface())
		sli = append(sli, v.MapIndex(k).Interface())
	}
	return sli
}

func SliceConvertMap(sli []any) map[string]interface{} {
	mapObj := make(map[string]any, 0)
	for i := 0; i < len(sli); i++ {
		if i+1 >= len(sli) {
			break
		}
		mapObj[sli[i].(string)] = sli[i+1]
		i++
	}
	return mapObj
}
