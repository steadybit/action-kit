# Parameter Types

Attacks' support parameters for configuration purposes. For example, you may want to use a parameter of type `duration` to configure how long an attack is supposed to take. This document explains what the supported parameter types are and how they work.

## `boolean`

Either `true` or `false` values. With optional support for `null` when `required=false` and no `defaultValue` is defined.

### Example

#### Parameter Definition

```json
{
  "name": "activateOrder66",
  "label": "Eradicate Jedi Order?",
  "type": "boolean",
  "defaultValue": "false"
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "activateOrder66": true
}
```

##### Without a Value

```json
{
  "activateOrder66": null
}
```

## `duration`

A time duration. Renders appropriate UI controls that facilitate time inputs—exposed as `number`s representing milliseconds to extensions.

### Example

#### Parameter Definition

```json
{
  "name": "jarJarBinksSongDuration",
  "label": "How long do you want to be bothered by Jar Jar Binks?",
  "type": "duration",
  "defaultValue": "0s"
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "jarJarBinksSongDuration": 5000 // milliseconds
}
```

##### Without a Value

```json
{
  "jarJarBinksSongDuration": null
}
```

## `integer`

Any integer number.

### Example

#### Parameter Definition

```json
{
  "name": "starWarsEpisode",
  "label": "What is your favorite Star Wars episode?",
  "type": "integer",
  "defaultValue": "4"
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "starWarsEpisode": 5
}
```

##### Without a Value

```json
{
  "starWarsEpisode": null
}
```

## `percentage`

`percentage` is a variation of the `integer` parameter that renders more appropriate user interface controls. A value of 0% is represented as the number `0`. 100% is represented as the number `100`.

### Example

#### Parameter Definition

```json
{
  "name": "deathStarEnergyLevel",
  "label": "How much should the Death Star be charged?",
  "type": "percentage",
  "defaultValue": "69"
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "deathStarEnergyLevel": 69
}
```

##### Without a Value

```json
{
  "deathStarEnergyLevel": null
}
```

## `string`

Strings are the most fundamental parameter type. They represent arbitrary character sequences just like you would expect.

**Note:** It is the responsibility of an extension to decide what to do when receiving an empty string.

### Example

#### Parameter Definition

```json
{
  "name": "fullName",
  "label": "Full Name",
  "type": "string",
  "defaultValue": "Jane Doe"
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "fullName": "Admiral Ackbar"
}
```

##### Without a Empty String

```json
{
  "fullName": ""
}
```

##### Without a Value

```json
{
  "fullName": "Jane Doe"
}
```

## `string[]` or `string_array`

You can use the `string[]` type for multiple textual inputs.

### Example

#### Parameter Definition

```json
{
  "name": "lightsaberCombatForm",
  "label": "Lightsaber Combat Form",
  "type": "string_array",
  "defaultValue": "[\"shii_cho\", \"ataru\"]",
  "options": [
    {
      "label": "Shii-Cho",
      "value": "shii_cho"
    },
    {
      "label": "Makashi",
      "value": "makashi"
    },
    {
      "label": "Soresu",
      "value": "soresu"
    },
    {
      "label": "Ataru",
      "value": "ataru"
    }
    // ...
  ]
}
```

#### Configuration Value Received in `prepare` Call of Attacks

##### With a Value
```json
{
  "lightsaberCombatForm": ["soresu"]
}
```

##### Without a Selected Input

```json
{
  "lightsaberCombatForm": []
}
```


##### Without any Input

```json
{
  "lightsaberCombatForm": null
}
```

## `password`

The `password` parameter behaves like the `string` parameter type, except for the visual presentation in the Steadybit user interface.
