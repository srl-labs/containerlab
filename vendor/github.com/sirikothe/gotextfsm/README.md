# gotextfsm
golang implementation of google's textfsm library

This is a golang version of google's textfsm library.

Textfsm is a template based state machine for parsing semi-formatted text. Originally developed to allow programmatic access to information returned from the command line interface (CLI) of networking devices.

The complete documentation about text fsm is given here: https://github.com/google/textfsm/wiki/TextFSM

The source code of textfsm in python is given here: https://github.com/google/textfsm

This porting attempts to be 100% compatible to the original textfsm specification given in the above links.

## Get Started
Get the library:
```
$ go get github.com/sirikothe/gotextfsm
```
### Basic Usage
Import the gotextfsm library
```
import "github.com/sirikothe/gotextfsm"
```
Create a TextFSM Object and parse the template
```
  fsm := gotextfsm.TextFSM{}
  # template should hold the string of the Textfsm to be parsed.
  err := fsm.ParseString(template)
  # err will be nil if the parsing of the template is successful. 
  # If will contain error object if the parsing failed.
```
Parse a raw input string using the fsm object created
```
  parser := gotextfsm.ParserOutput{}
  err = parser.ParseTextString(input, fsm, true)
  # err will be nil if the parsing of the input (as per the fsm provided) is successful. 
  # If will contain error object if the parsing failed.
```
At the end of the parsing of input, the parser's Dict object contains the results



### Complete Example code:
```
package main

import (
	"fmt"

	"github.com/sirikothe/gotextfsm"
)

func main() {
	template := `Value beer (.*)

Start
  ^$beer
`
	input := "Hello World"
	fsm := gotextfsm.TextFSM{}
	err := fsm.ParseString(template)
	if err != nil {
		fmt.Printf("Error while parsing template '%s'\n", err.Error())
		return
	}
	parser := gotextfsm.ParserOutput{}
	err = parser.ParseTextString(input, fsm, true)
	if err != nil {
		fmt.Printf("Error while parsing input '%s'\n", err.Error())
	}
	fmt.Printf("Parsed output: %v\n", parser.Dict)
}
```
## How to read results of parsing
The defined type for ParserOutput.Dict is `[]map[string]interface{}`.

This is an array of maps. Each element in array represents a record of parsed output.

Each record is of type `map[string]interface{}`, where the key is the field name (type string) and value is the field value (type interface{})

Even though the field value is defined as type `interface{}`, its concrete type is one of
* `[]string` type -> For variables declared as `List` in the definition. ex. `Value List ifnames (\w+)`
* `map[string]string` type -> For variables declared as scalar, but with nested regexes. ex. `Value person ((?P<name>\w+):\s+(?P<age>\d+)\s+(?P<state>\w{2})\s*)`
* `[]map[string]string` type -> For variables declared as List, but with nested regexes. ex. `Value List person ((?P<name>\w+):\s+(?P<age>\d+)\s+(?P<state>\w{2})\s*)`
* `string` type -> For every other variable type. This is most common use case.
#### Example code to handle the output - Option 1
Following complete code snippet shows an example of how to process the output of parser.
```
package main

import (
	"fmt"

	"github.com/sirikothe/gotextfsm"
)

func main() {
	template := `Value continent (.*)
Value List countries (.*)
Value state_abbr ((?P<fullstate>\w+):\s+(?P<abbr>\w{2}))
Value List persons ((?P<name>\w+):\s+(?P<age>\d+)\s+(?P<state>\w{2})\s*)

Start
	^Continent: ${continent}
	^Country: ${countries}
	^State: ${state_abbr}
	^${persons}
`
	input := `Continent: North America
Country: USA
Country: Candada
Country: Mexico
State: California: CA
Siri: 50 CA
Raj: 22 NM
Gandhi: 150 NV
`

	fsm := gotextfsm.TextFSM{}
	err := fsm.ParseString(template)
	if err != nil {
		fmt.Printf("Error while parsing template '%s'\n", err.Error())
		return
	}
	parser := gotextfsm.ParserOutput{}
	err = parser.ParseTextString(input, fsm, true)
	if err != nil {
		fmt.Printf("Error while parsing input '%s'\n", err.Error())
	}
	for _, record := range parser.Dict {
		for key, value := range record {
			switch value.(type) {
			case string:
				// typecast to string and use it.
				// ex: Value continent (.*)
				fmt.Printf("%s: %s\n", key, value.(string))
			case []string:
				// List type variable. typecast to []string and use it.
				// ex: Value List countries (.*)
				fmt.Printf("%s: %s\n", key, value.([]string))

			case map[string]string:
				// Nested scalar variable.
				// ex: Value state_abbr ((?P<fullstate>\w+):\s+(?P<abbr>\w{2}))
				fmt.Printf("%s: %s\n", key, value.(map[string]string))

			case []map[string]string:
				// Nested List variable.
				// ex: Value List persons ((?P<name>\w+):\s+(?P<age>\d+)\s+(?P<state>\w{2})\s*)
				fmt.Printf("%s: %s\n", key, value.([]map[string]string))
			default:
				// Shoule never happen.
				panic("Really?")
			}
		}
	}
}
```

### Example code to handle the output - Option 2
You can also marshal the resulting dict to json (or yaml) if that make is easier for you to handle the output.
```
	str, err := json.Marshal(parser.Dict)
	if err != nil {
		fmt.Printf("Unable to convert dict to json \n", err)
	} else {
		fmt.Printf("JSON: %s\n", str)
	}
```
Output from the above example:
```
JSON: [{"continent":"North America","countries":["USA","Candada","Mexico"],"persons":[{"age":"50","name":"Siri","state":"CA"},{"age":"22","name":"Raj","state":"NM"},{"age":"150","name":"Gandhi","state":"NV"}],"state_abbr":{"abbr":"CA","fullstate":"California"}}]
```

## Highlights:

* Attempts to be 100% compatible with textfsm original textfsm implementation (See differences section)
* Very nimble code with zero external dependencies on any other libraries.
* Well tested (~ 97% code coverage) *>1740 Test cases executed!!!*
    * All test cases of python's implementation are ported and executed
    * More test cases added as well to test corner cases
* All the test cases of ntc-templates are executed.
	* Out of 1578 test cases of ntc-templates, 28 of them are failing (All of them due to reasons listed in `Caveats` Section)
(https://github.com/networktocode/ntc-templates)

## Differences with Python's implementation
Following are the differences between this implementation of TextFSM and original implementation of Python:
* Python's implementation provides 2 ways of getting results.
    * Output as a list of lists (outer list represents a record and inner list contains the values. - in the order the Values declared)
    * Output as a list of dicts.
  This implementation provides the output as only slice of maps. It does not provide as slice of slices.
* [TODO]This implementation (currently) implements the core TextFSM Functionality. It does not implement the following 
    * clitable
    * terminal
    * texttable

## Caveats
There are differences in golang's regular expression implementation and python's.
Because of this reason, some templates that are valid in python fail parsing in gotextfsm.
Following are some of the known examples:
* golang restricts repeat cound to be 1000 while python allows it to be 2^^16 -1. Because of this reason, the regular expressions like `Value SAP_COUNT ([0-9]{1,1500})` throw an error.
More details about this are discussed at https://github.com/golang/go/issues/7252
* golang does not support perl syntax like `(?<`. The regex like `Value NAME (\S.*(?<!\s))` throws an error
## Testing
```
PS C:\Users\siri\code\nuviso\GitHub\gotextfsm> go test -v
=== RUN   TestParseText
    parsetext_test.go:83: Executed 48 test cases
--- PASS: TestParseText (0.04s)
=== RUN   TestPyTemplate
    pytemplate_test.go:31: Executed 9 test cases
--- PASS: TestPyTemplate (0.00s)
=== RUN   TestRuleParse
    rule_test.go:41: Executed 25 test cases
--- PASS: TestRuleParse (0.00s)
    textfsm_test.go:84: Executed 56 test cases
--- PASS: TestFSMParse (0.01s)
=== RUN   TestValueParse
    value_test.go:56: Executed 27 test cases
--- PASS: TestValueParse (0.01s)
PASS
ok      _/C_/Users/siri/code/nuviso/GitHub/gotextfsm    0.235s
PS C:\Users\siri\code\nuviso\GitHub\gotextfsm> go test -cover
PASS
coverage: 96.9% of statements
ok      _/C_/Users/siri/code/nuviso/GitHub/gotextfsm    0.236s
```
