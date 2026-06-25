---
applyTo: 'docs/**/*.md'
description: 'Guidelines for writing and reviewing Kyma documentation. Covers style, terminology, formatting, templates, and content structure for technical documentation in the docs/ folder.'
---

# Documentation Writing Instructions

Follow these rules when creating, editing, or reviewing documentation
in the `docs/` folder. These guidelines are based on the
[Kyma style and terminology guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/04-style-and-terminology.md)
and the
[Kyma formatting guidelines](https://github.com/kyma-project/community/blob/main/docs/guidelines/content-guidelines/03-formatting.md).

## General Rules

- Write clear, precise, and easy-to-read documentation
- Use **active voice** and **present tense**; avoid passive voice and future tense
- Use **imperative mood** for instructions (no "please")
- Address readers as **"you"**, not "we" or "let's"
- State the purpose first, then the instruction: "To [purpose], [instruction]."
- State the condition first, then the instruction: "If [condition], [instruction]."
- Use full sentences to introduce lists
- All list items must follow a consistent pattern; they must not be fragments of a running sentence

### Word Choice

Avoid ambiguous words. Prefer unambiguous alternatives:

| Instead of | Use |
|---|---|
| "as" or "since" (causation) | "because" |
| "as" or "since" (temporal) | "while" |
| "as" (comparison) | "like" |
| "may not" (possibility) | "might not" |
| "may not" (prohibition) | "must not" |
| "should" (recommendation) | "we recommend" |
| "should" (requirement) | "must" |
| "once" (temporal/conditional) | "after" or "when" |
| "i.e." | "that means" |
| "e.g." | "for example", "such as", "like" |
| "allows you to", "enables you to" | "you can" |
| "leverage", "utilize" | "use" |
| "via" | "using" or "with" |
| "integrate/integration" | "connect/connection" |

- Use "can" for ability, "may" for permission
- Use "must" for mandatory requirements
- Avoid marketing lingo and unnecessary jargon

## Style and Terminology

- Use **American English** spelling
- Use **sentence case** for standard text
- Use **Title Case** for component names and headings
- Use **CamelCase** for Kubernetes resources (for example, `ConfigMap`, `APIRule`)
- Always capitalize "Kubernetes"; never abbreviate it
- Do not capitalize "namespace"
- Use serial commas (Oxford commas)
- Avoid parentheses; use lists instead

## Formatting

- Use **bold** for: parameters, HTTP headers, events, roles, UI elements, variables, and placeholders
- Use `code font` for: code examples, values, endpoints, filenames, paths, repository names, status codes, flags, and custom resources
- Use **ordered lists** for sequential procedures
- Use **unordered lists** for non-sequential items
- Keep list items consistent in structure (all sentences, all fragments, or all questions — never mixed)
- Use action verbs and present tense in headings (for example, "Set Up the OTLP Input")
- Use tables for comparisons and structured information
- Break lengthy paragraphs into lists or tables for readability

### Callout Panels

Use GitHub-flavored Markdown callouts:

- `> [!NOTE]` — for specific information the reader should be aware of
- `> [!WARNING]` — for critical alerts that could cause problems
- `> [!TIP]` — for helpful advice and best practices

## Document Templates

Use the templates from the [kyma-project template repository](https://github.com/kyma-project/template-repository/tree/main/docs/user/assets/templates):

| Template | Use for |
|---|---|
| `concept.md` | Explaining foundational ideas and principles |
| `task.md` | Step-by-step instructions and how-to guides |
| `troubleshooting.md` | Diagnostic information and solutions to common issues |
| `custom-resource.md` | Documenting custom resource configuration and usage |

## Review Checklist

When reviewing documentation changes, verify:

- [ ] Active voice and present tense throughout
- [ ] Imperative mood for all instructions
- [ ] No ambiguous words (as, since, once, should, may not)
- [ ] No marketing lingo (allows, enables, leverage, utilize)
- [ ] Purpose before instruction, condition before conclusion
- [ ] Lists introduced with full sentences
- [ ] Consistent list item structure
- [ ] Correct use of bold vs. code font
- [ ] Serial commas used consistently
- [ ] Headings use Title Case with action verbs
- [ ] Appropriate callout panels for notes, warnings, tips
- [ ] American English spelling
- [ ] "Kubernetes" capitalized, "namespace" lowercase
- [ ] CamelCase for Kubernetes resources
