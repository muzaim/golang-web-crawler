# Spec Delta

## Purpose

Defines what information the crawler extracts from each fetched page and how non-HTML or failed responses are represented in the page result.

## ADDED Requirements

### Requirement: Page result fields
Every fetched URL SHALL produce one page result with these fields: URL, depth, HTTP status code, title, meta description, internal links, external links, and error (empty on success).

#### Scenario: Successful HTML page
- **WHEN** a URL returns status 200 with `Content-Type: text/html`
- **THEN** its result contains the URL, depth, status 200, extracted title, meta description, link lists, and an empty error

### Requirement: Title extraction
The crawler SHALL use the trimmed text content of the document's first `<title>` element as the page title. The title SHALL be empty when the document has no `<title>` element.

#### Scenario: Title with whitespace
- **WHEN** the page contains `<title>\n  Hello World  \n</title>`
- **THEN** the result title is `Hello World`

#### Scenario: Missing title
- **WHEN** the page has no `<title>` element
- **THEN** the result title is empty

### Requirement: Meta description extraction
The crawler SHALL use the trimmed `content` attribute of the first `<meta name="description">` element as the meta description. The `name` attribute is matched case-insensitively.

#### Scenario: Meta description present
- **WHEN** the page contains `<meta name="Description" content=" A sample site ">`
- **THEN** the result meta description is `A sample site`

### Requirement: Link extraction and classification
The crawler SHALL collect the `href` of every `<a>` element and resolve each one against the page URL, honoring a `<base href>` if present. It SHALL normalize each link and classify it as internal (same host as the seed) or external. Each list SHALL contain no duplicates.

#### Scenario: Relative and absolute links classified
- **WHEN** a page at `https://example.com/docs/` on seed host `example.com` links to `intro`, `/about`, and `https://github.com/x`
- **THEN** internal links are `https://example.com/docs/intro` and `https://example.com/about`, and the external link is `https://github.com/x`

#### Scenario: Duplicate links on one page
- **WHEN** a page links to `/about` three times
- **THEN** `https://example.com/about` appears once in the internal links

#### Scenario: Empty or fragment-only href
- **WHEN** a page contains `<a href="">` or `<a href="#top">`
- **THEN** the link resolves to the page itself and is not fetched again

### Requirement: Non-HTML responses are not parsed
When a response's `Content-Type` is not `text/html`, the crawler SHALL record the status code, leave title, description, and links empty, and SHALL NOT parse the body.

#### Scenario: PDF response
- **WHEN** a URL returns `Content-Type: application/pdf`
- **THEN** its result has the status code, empty title and links, and no links from it are followed

### Requirement: Failed fetches are reported
When a request fails with a network error, timeout, or rejected redirect, the crawler SHALL record the error in the page result and continue crawling other URLs. Non-2xx responses SHALL record their status code and SHALL NOT have links extracted.

#### Scenario: 404 page
- **WHEN** a URL returns status 404
- **THEN** its result has status 404, no links are followed from it, and the crawl continues

#### Scenario: Connection refused
- **WHEN** a URL cannot be connected to
- **THEN** its result has a non-empty error and status 0
