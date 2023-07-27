package sync

import "log"

func Go(f func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("recover: %v\n", err)
			}
		}()
		f()
	}()
}
