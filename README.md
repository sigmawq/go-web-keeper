# Web keeper

Web keeper is a tool for periodically querying and saving web resources. Much like a simplified version of web archive.

## Installation
`go build .`

## Usage
1. Create `config.json` in the root directory. You can use the template from this repo.
2. Specify one or many URL jobs. A single URL job will parse a web resource with many of it's sources and save it to the database.
3. If `server` object is present in the config than a server will be set up. User can use this server to query stored pages in the database.
4. You can also query pages via a CLI.

## Query
Option 1 - CLI. 
* Get data in range: `GetData <url> <from> <to>`. Where `<from>` and `<to>` are in a specific format (more on this below)
* Get all data: `GetData <url> all`

Option 2 - HTTP Get.
*   Make sure server setup is set up. 
*   Perform an HTTP get and save the file: 
*   Get data in range: `curl -o filename.zip "address:port/data/protocol/URL/from/to"`
*   Get all data: `curl -o filename.zip "address:port/data/protocol/URL/all"`

## Date format
There is a specific format for all dates used in web keeper: Mon Jan _2 15:04:05 2006
Date is given as a specific timestamp: `Mon Jan 2 15:04:05 MST 2006` (Same as golang `time` package does)

## License
[MIT](https://choosealicense.com/licenses/mit/)