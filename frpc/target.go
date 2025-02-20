package frpc

import (
	"fmt"
)

func BuildEtcdTarget(target string) string {
	return fmt.Sprintf("%s://%s", EtcdScheme, target)
}
