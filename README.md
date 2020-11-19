Webcrawler
==========

A simple PoC of a webcrawler that starting from an initial URL visits all URLs
it finds on the domain as long as they belong the subdomain.
A detailed tutorial on the realization can be found at
[webcrawler-from-scratch](https://codepr.github.io/webcrawler-from-scratch)

**Features**

- Concurrent
- Deduplicate, tries not to crawl the same URLs more than once
- Respect robots.txt directives
- Tries to be polite, if no crawling delay is found on the robots.txt it
  generate a randomized delay based on the response time of the server and a
  fixed value passed in as configuration
- Allow to specify a list of exclusion, links to avoid based on their extension

**Dependencies**

- [goquery](https://github.com/PuerkitoBio/goquery) to easily parse HTML
  documents, gives convenient JQuery-like filtering methods, the alternative
  would've been walking down the document tree using the go std library
- [rehttp](https://github.com/PuerkitoBio/rehttp) enable easy retry on HTTP
  errors, with retry number settings and an exponential backoff between each
  try
- [robotstxt](github.com/temoto/robotstxt) allow to efficiently parse
  `robots.txt` files on the root of each domain

The project can be built with

```sh
go build -o . ./...
```

There's a bunch of tests for the most critical parts
```sh
go test -v ./...
```

and run

```sh
./webcrawler -target https://golang.org -concurrency 4 -depth 8
```

it's possible to set most of the crawler settings by ENV variables:

- `USERAGENT` it's the User-Agent header we want to display
- `CRAWLING_TIMEOUT` the number of seconds to wait for exiting crawling a page
  after the last link found
- `CONCURRENCY` the number of worker goroutines to run in parallel while
  fetching websites; 0 means unlimited
- `MAX_DEPTH` the number of links to fetch for each level; 0 means unbounded
- `FETCHING_TIMEOUT` the timeout to wait if a fetch isn't responding
- `POLITENESS_DELAY` the fixed delay to wait between multiple calls under the
  same domain

Supports extension exclusion from the crawl and some degree of politeness,
checks for `/robots.txt` directives, if not found it assumes all subdomains are
valid and tries to adjust a random delay for each call:

`delay = max(random(.5 * fixedDelay < x < 1.5 * fixedDelay), robots-delay, lastResponse time ** 2)`

The main crawling function consumes from a channel in a loop all the links to
crawl, spawning goroutine workers to fetch new links on every page, limiting
the concurrency with a semaphore. Every worker respect a delay between
multiple calls to avoid flooding the target webserver.
There's no recursion involved, making it quiet efficient and allowing for
high levels of depth.

## Decisions

Originally I thought to design and implement the crawler as a simple
microservices architecture, decoupling the fetching from the presentation
service and using some queues to communicate asynchronously, `RabbitMQ` was
taken into consideration for that.

I eventually decided to produce a simpler PoC explaining here weak points and
improvements that could be made.

The application is entirely ephemeral, this means that stopping it will loose
any progression on the crawling job. I implemented the core features trying to
decouple responsibilities as much as possible in order to make it easier to
plug different components:

- A `crawler` package which contains the crawling logic
    - `crawlingrules` defines a simple ruleset to follow while crawling, like
      robots.txt rules and delays to respect
- A `messaging` package which offer a communication interface, used to push
  crawling results to different consumers, currently the only consumer is a
  simple goroutine that prints links found
- `fetcher` is a package dedicated to the HTTP communication and parsing of
  HTML content, `Parser` and `Fetcher` interfaces allow to easily implement
  multiple solutions with different underlying backend libraries and behaviors

### Known issues

- No "checkpoint" persistence-like to graceful pause/restart the process
- Deduplication could be better, no `rel=canonical` handling, doesn't check
  for `http` vs `https` version of the site when they display the same contents
- `Retry-After` header is not respected after a 503 response
- 429 response as well is not considered
- It's simple, no session handling/cookies
- Logging is pretty simple, no external libraries, just print errors
- Doesn't implement a sanitization of input except for missing scheme,
  if a domain requires `www` it cannot be omitted, otherwise it'll tries
  to contact the server with no succes, in other words it requires a correct
  URL as input

### Improvements

There's plenty of room for improvements for a production-ready solution, mostly
dependents of the purpose of the software:

- REST interface to ingest jobs and query, probably behind a load-balancer
- crawling logic with persistent state, persistent queue for links to crawl
- configurable logging
- extensibility and customization of the crawling rules, for example pluggable
  delay functions for each domain
- better definition of errors and maybe a queue to notify them/gather them by
  stderr through some kind of aggregation stack (e.g. ELK)
- reverse index generation and content signature generation, in order to avoid
  crawling of different links pointing to the same (or mostly similar)
  contents. Reverse index service should expose APIs to query results obtaining
  a basic search engine.
- **depending on the final purpose** separate even more the working logic from
  the business logic, probably adding a job-agnostic worker package
  implementing a shared nothing actor-model based on the goroutines which can
  be reused for different purposes as the project grows
