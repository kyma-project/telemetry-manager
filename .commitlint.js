// commitlint rules documentation: https://commitlint.js.org/#/reference-rules
//
// Each PR encompasses these sections:
//     type(scope?): subject
//     body?
//     footer?

module.exports = {
    rules: {
        // The scope clause must always be in lowercase.
        "scope-case": [2, "always", "lower-case"],

        // The subject-case clause can be of any case to allow mentioning abbreviations and K8s resources.
        // "subject-case": [2, "always", ["sentence-case", "start-case", "pascal-case", "upper-case"]],

        // The subject-case clause mustn't be empty and mustn't end with a full stop.
        "subject-empty": [2, "never"],
        "subject-full-stop": [2, "never", "."],

        // The subject-case clause's length must be between 5 and 120 characters.
        "subject-max-length": [2, "always", "120"],
        "subject-min-length": [2, "always", "5"],

        // The type must always be in lowercase and be one of the predefined values.
        "type-enum": [2, "always", ["feat", "fix", "docs", "refactor", "test"]],
        "type-case": [2, "always", "lower-case"],
    }
}
