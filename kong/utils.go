package kong

import (
	"log"
	"regexp"
	"time"
)

var computedPluginProperties = []string{"created_at", "id", "consumer", "service", "route"}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}

func getRegex(r *regexp.Regexp, err error) *regexp.Regexp {
	return r
}

func readIntArrayFromInterface(in interface{}) []int {
	if arr := in.([]interface{}); arr != nil {
		array := make([]int, len(arr))
		for i, x := range arr {
			item := x.(int)
			array[i] = item
		}

		return array
	}

	return []int{}
}

func retryOnce(f func() error) error {
	return retry(1, f)
}

func retry(retries int, f func() error) error {
	attempt := 1

	for {
		if err := f(); err != nil {
			if attempt > retries {
				return err
			}
			log.Printf("Attempt %d failed: %v, retry in 1 second...", attempt, err)
			time.Sleep(1 * time.Second)
			attempt++
			continue
		}
		return nil
	}
}
