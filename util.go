/**
 * File: util.go
 * Author: Ming Cheng<mingcheng@outlook.com>
 *
 * Created Date: Thursday, June 23rd 2022, 8:41:25 pm
 * Last Modified: Thursday, July 7th 2022, 6:31:41 pm
 *
 * http://www.opensource.org/licenses/MIT
 */

package socks5lb

import (
	"os"
	"strings"
)

func GetEnv(name, def string) string {
	result := os.Getenv(name)
	if result == "" {
		result = def
	}

	return strings.TrimSpace(result)
}
