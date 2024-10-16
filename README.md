# krknctl
Krkn Terminal UI & CLI

## Dev Blog
As this project is actively evolving, consider this README as more of a _Development Blog_ to showcase current features under development.

## Scenario Metadata
### Online Mode
In online mode, `krknctl` leverages the `quay.io` API to retrieve scenario metadata. This metadata describes scenarios, enables autocomplete for available scenarios, and validates input. Nothing is stored locally, and no custom APIs are used; everything is based on OCI standards. The image metadata is labeled and accessed through the Quay.io image inspection API.

### Offline Mode
_Work In Progress_

## Command Autocompletion

Command autocompletion is powered by the [Cobra Command](https://github.com/spf13/cobra) Library. Metadata is retrieved as described above, and the autocompletion feature is installed via the `completion` command. It is compatible with most popular *nix shells:

![command autocomplete](media/autocomplete.gif)

### `describe` Scenario

Displays details of a scenario and its specific flags:
![describe scenario](media/describe.gif)

### `list` Available Scenarios

Shows all scenarios available in the registry:
![list scenarios](media/list.gif)

### `run` Scenario

Executes a scenario image with the necessary flags (translated as the respective container environment variables), validating the inputs against the type schema defined within the image manifest.

#### Type Schema
The type schema is a simple typing system designed to validate user input. Here is an example of the schema format:

```json
[
    {
        "name":"cpu-percentage",
        "shortDescription":"CPU percentage",
        "description":"Percentage of total CPU to be consumed",
        "variable":"TOTAL_CHAOS_DURATION",
        "type":"number",
        "required":"true"
    },
    {
        "name":"namespace", 
        "shortDescription":"Namespace",
        "description":"Namespace where the scenario container will be deployed",
        "variable":"NAMESPACE",
        "type":"string",
        "default":"default"
    }
]
```

Supported types include:
- `number`
- `bool`
- `string`
- `enum`
- `base64`
- `file`

Key features for each type:
- **All types**:
    - Support for default values and required fields
    - Upcoming features: dependency on another field (`requires`) and mutual exclusion (`mutuallyExcludes`)
- **String**:
    - Regex validation
- **Enum**:
    - Specific allowed values

Each schema element corresponds to a command flag in the format `--<field_name>`

![run flags](media/validation.gif)

input is validated against this schema:

![run flags](media/input-validation.gif)

## Container Runtime Integration

### Podman
_Work In Progress_

### Docker
_Work In Progress_