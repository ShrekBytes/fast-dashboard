package dashdashdash

import (
	"crypto/tls"
	"encoding/json"
	"errors"

	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

var (
	errNoContent      = errors.New("failed to retrieve any content")
	errPartialContent = errors.New("failed to retrieve some of the content")
)

const defaultClientTimeout = 5 * time.Second

var defaultHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 10,
		Proxy:               http.ProxyFromEnvironment,
	},
	Timeout: defaultClientTimeout,
}


// monitorHTTPClient has no global timeout — monitor requests use per-request
// context timeouts so that user-configured timeout values (default 7s) aren't
// silently clipped by the 5s client timeout.
var monitorHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 10,
		Proxy:               http.ProxyFromEnvironment,
	},
}

var monitorInsecureHTTPClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Proxy:           http.ProxyFromEnvironment,
	},
}

type requestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

var userAgentString = "dash-dash-dash/" + buildVersion

func decodeJsonFromRequest[T any](client requestDoer, request *http.Request) (T, error) {
	var result T

	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		truncatedBody, _ := limitStringLength(string(body), 256)

		return result, fmt.Errorf(
			"unexpected status code %d from %s, response: %s",
			response.StatusCode,
			request.URL,
			truncatedBody,
		)
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}


type workerPoolTask[I any, O any] struct {
	index  int
	input  I
	output O
	err    error
}

type workerPoolJob[I any, O any] struct {
	data    []I
	workers int
	task    func(I) (O, error)
}

const defaultNumWorkers = 10

func (job *workerPoolJob[I, O]) withWorkers(workers int) *workerPoolJob[I, O] {
	if workers == 0 {
		job.workers = defaultNumWorkers
	} else {
		job.workers = min(workers, len(job.data))
	}

	return job
}

func newJob[I any, O any](task func(I) (O, error), data []I) *workerPoolJob[I, O] {
	return &workerPoolJob[I, O]{
		workers: defaultNumWorkers,
		task:    task,
		data:    data,
	}
}

func workerPoolDo[I any, O any](job *workerPoolJob[I, O]) ([]O, []error, error) {
	results := make([]O, len(job.data))
	errs := make([]error, len(job.data))

	if len(job.data) == 0 {
		return results, errs, nil
	}

	if len(job.data) == 1 {
		results[0], errs[0] = job.task(job.data[0])
		return results, errs, nil
	}

	tasksQueue := make(chan *workerPoolTask[I, O])
	resultsQueue := make(chan *workerPoolTask[I, O])

	var wg sync.WaitGroup

	for range job.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for t := range tasksQueue {
				t.output, t.err = job.task(t.input)
				resultsQueue <- t
			}
		}()
	}

	go func() {
		for i := range job.data {
			tasksQueue <- &workerPoolTask[I, O]{
				index: i,
				input: job.data[i],
			}
		}
		close(tasksQueue)
		wg.Wait()
		close(resultsQueue)
	}()

	for task := range resultsQueue {
		errs[task.index] = task.err
		results[task.index] = task.output
	}

	return results, errs, nil
}
