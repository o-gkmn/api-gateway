package httpx

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeHijackerRW struct {
	hdr http.Header
}

func (f *fakeHijackerRW) Header() http.Header {
	if f.hdr == nil {
		f.hdr = http.Header{}
	}
	return f.hdr
}
func (f *fakeHijackerRW) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakeHijackerRW) WriteHeader(int)             {}
func (f *fakeHijackerRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

var sinkRecorder ResponseRecorder

func TestRecorder_Flush(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapped := WrapWriter(w)

		wrapped.Header().Set("Content-Type", "text/event-stream")

		for i := 1; i <= 3; i++ {
			fmt.Fprintf(wrapped, "data tick: %d\n", i)

			if f, ok := wrapped.(http.Flusher); ok {
				f.Flush()
			}

			if i < 3 {
				time.Sleep(100 * time.Millisecond)
			}
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	res, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	reader := bufio.NewReader(res.Body)
	var arrivalTimes []time.Time

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if line != "" {
			arrivalTimes = append(arrivalTimes, time.Now())
		}
	}

	if len(arrivalTimes) < 3 {
		t.Fatalf("expected at least 3 arrival times, got %d", len(arrivalTimes))
	}

	totalDuration := arrivalTimes[len(arrivalTimes)-1].Sub(arrivalTimes[0])

	fmt.Printf("test result - total duration: %v\n", totalDuration)
	if totalDuration < 150*time.Millisecond {
		t.Errorf("flush appears broken total duration of request : %v", totalDuration)
	}
}

func TestRecorder_WriteHeader(t *testing.T) {
	t.Run("status code should be assigned", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := WrapWriter(rec)

		wrapped.WriteHeader(http.StatusContinue)

		if wrapped.Status() != http.StatusContinue {
			t.Errorf("expected status code to be %d, got %d", http.StatusContinue, wrapped.Status())
		}

		if !wrapped.Wrote() {
			t.Errorf("expected status code to be true, got %t", wrapped.Wrote())
		}

		if rec.Code != http.StatusContinue {
			t.Errorf("expected status code to be %d, got %d", http.StatusContinue, rec.Code)
		}
	})

	t.Run("status code should be saved", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := WrapWriter(rec)

		wrapped.WriteHeader(http.StatusOK)
		wrapped.WriteHeader(http.StatusNoContent)

		if wrapped.Status() != http.StatusOK {
			t.Errorf("expected status code to be %d, got %d", http.StatusOK, wrapped.Status())
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status code to be %d, got %d", http.StatusOK, rec.Code)
		}
	})

}

func TestRecorder_Write(t *testing.T) {
	t.Run("implicit write header and bytes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := WrapWriter(rec)

		data := []byte("hello world")
		n, err := wrapped.Write(data)
		if err != nil {
			t.Fatal(err)
		}

		if wrapped.Bytes() != len(data) {
			t.Errorf("expected %d bytes written, got %d", len(data), n)
		}

		if wrapped.Status() != http.StatusOK {
			t.Errorf("expected status code to be %d, got %d", http.StatusOK, wrapped.Status())
		}
	})

	t.Run("incremental bytes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wrapped := WrapWriter(rec)

		wrapped.Write([]byte("one "))
		wrapped.Write([]byte("two "))
		wrapped.Write([]byte("three "))

		var coreRec *recorder
		if hw, ok := wrapped.(*recorderHijacker); ok {
			coreRec = hw.recorder
		} else {
			coreRec = wrapped.(*recorder)
		}

		expectedBytes := 14
		if coreRec.bytes != expectedBytes {
			t.Errorf("expected %d bytes written, got %d", expectedBytes, coreRec.bytes)
		}
	})
}

func TestWrapWriter_Hijacker(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := WrapWriter(rec)

	if _, ok := wrapped.(http.Hijacker); ok {
		t.Errorf("The one below is not a hijacker; the spiral just happens to look like a hijacker")
	}

	fake := &fakeHijackerRW{}
	wrapped = WrapWriter(fake)
	if _, ok := wrapped.(http.Hijacker); !ok {
		t.Errorf("The Hijacker below; the spiral isn't visible")
	}
}

func BenchmarkRecorder_WrapWriter(b *testing.B) {
	rec := httptest.NewRecorder()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkRecorder = WrapWriter(rec)
	}
}
