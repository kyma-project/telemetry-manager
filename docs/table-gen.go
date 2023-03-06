package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

type FunctionSpecGenerator struct {
	elementsToKeep map[string]string
	elementsToSkip map[string]bool
}

const (
	FunctionSpecIdentifier      = `FUNCTION-SPEC`
	REFunctionSpecPattern       = `(?s)<!--\s*` + FunctionSpecIdentifier + `-START\s* -->.*<!--\s*` + FunctionSpecIdentifier + `-END\s*-->`
	KeepThisIdentifier          = `KEEP-THIS`
	REKeepThisPattern           = `[^\S\r\n]*[|]\s*\*{2}([^*]+)\*{2}.*<!--\s*` + KeepThisIdentifier + `\s*-->`
	SkipIdentifier              = `SKIP-ELEMENT`
	RESkipPattern               = `<!--\s*` + SkipIdentifier + `\s*([^\s]+)\s*-->`
	SkipWithAncestorsIdentifier = `SKIP-WITH-ANCESTORS`
	RESkipWithAncestorsPattern  = `<!--\s*` + SkipWithAncestorsIdentifier + `\s*([^\s-]+)\s*-->`
)

var (
	CRDFilename string
	MDFilename  string
	APIVersion  string
)

func main() {
	flag.StringVar(&CRDFilename, "crd-filename", "", "Full or relative path to the .yaml file containing crd")
	flag.StringVar(&MDFilename, "md-filename", "", "Full or relative path to the .md file containing the file where we should insert table rows")
	flag.StringVar(&APIVersion, "api-version", "v1alpha1", "API version your operattor uses")
	flag.Parse()

	toKeep := getElementsToKeep()
	toSkip := getElementsToSkip()
	generator := CreateFunctionSpecGenerator(toKeep, toSkip)
	doc := generator.generateDocFromCRD()
	replaceDocInMD(doc)
	print(doc)
}

func getElementsToKeep() map[string]string {
	inDoc, err := os.ReadFile(MDFilename)
	if err != nil {
		panic(err)
	}

	reFunSpec := regexp.MustCompile(REFunctionSpecPattern)
	funSpecPart := reFunSpec.FindString(string(inDoc))
	reKeep := regexp.MustCompile(REKeepThisPattern)
	rowsToKeep := reKeep.FindAllStringSubmatch(funSpecPart, -1)

	toKeep := map[string]string{}
	for _, pair := range rowsToKeep {
		rowContent := pair[0]
		paramName := pair[1]
		toKeep[paramName] = rowContent
	}
	return toKeep
}

func getElementsToSkip() map[string]bool {
	inDoc, err := os.ReadFile(MDFilename)
	if err != nil {
		panic(err)
	}

	doc := string(inDoc)
	reSkip := regexp.MustCompile(RESkipPattern)
	toSkip := map[string]bool{}

	pairsToParamsToSkip(toSkip, reSkip.FindAllStringSubmatch(doc, -1), false)

	reSkipWithAncestors := regexp.MustCompile(RESkipWithAncestorsPattern)
	pairsToParamsToSkip(toSkip, reSkipWithAncestors.FindAllStringSubmatch(doc, -1), true)

	return toSkip
}

func replaceDocInMD(doc string) {
	inDoc, err := os.ReadFile(MDFilename)
	if err != nil {
		panic(err)
	}

	newContent := strings.Join([]string{
		"<!-- " + FunctionSpecIdentifier + "-START -->",
		doc + "<!-- " + FunctionSpecIdentifier + "-END -->",
	}, "\n")
	re := regexp.MustCompile(REFunctionSpecPattern)
	outDoc := re.ReplaceAll(inDoc, []byte(newContent))

	outFile, err := os.OpenFile(MDFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()
	outFile.Write(outDoc)
}

func CreateFunctionSpecGenerator(toKeep map[string]string, toSkip map[string]bool) FunctionSpecGenerator {
	return FunctionSpecGenerator{
		elementsToKeep: toKeep,
		elementsToSkip: toSkip,
	}
}

func (generator *FunctionSpecGenerator) generateDocFromCRD() string {
	input, err := os.ReadFile(CRDFilename)
	if err != nil {
		panic(err)
	}

	// why unmarshalling to CustomResource don't work?
	var obj interface{}
	if err := yaml.Unmarshal(input, &obj); err != nil {
		panic(err)
	}

	docElements := map[string]string{}
	versions := getElement(obj, "spec", "versions")

	// move to another function to reduce 2inn
	for _, version := range versions.([]interface{}) {
		name := getElement(version, "name")
		if name.(string) != APIVersion {
			continue
		}
		functionSpec := getElement(version, "schema", "openAPIV3Schema", "properties", "spec")
		mergeMaps(docElements, generator.generateElementDoc(functionSpec, "spec", true, ""))
	}

	mergeMaps(docElements, generator.elementsToKeep)

	var doc []string
	for _, propName := range sortKeys(docElements) {
		doc = append(doc, docElements[propName])
	}

	return strings.Join(doc, "\n")
}

func (generator *FunctionSpecGenerator) generateElementDoc(obj interface{}, name string, required bool, parentPath string) map[string]string {
	result := map[string]string{}
	element := obj.(map[string]interface{})
	elementType := element["type"].(string)
	description := ""
	if d := element["description"]; d != nil {
		description = d.(string)
	}

	fullName := fmt.Sprintf("%s%s", parentPath, name)
	skipWithAncestors, shouldBeSkipped := generator.elementsToSkip[fullName]
	if shouldBeSkipped && skipWithAncestors {
		return result
	}
	_, isRowToKeep := generator.elementsToKeep[fullName]
	if !shouldBeSkipped && !isRowToKeep {
		result[fullName] = generateTableRow(fullName, required, description, name)
	}

	if elementType == "object" {
		mergeMaps(result, generator.generateObjectDoc(element, name, parentPath))
	}
	return result
}

func (generator *FunctionSpecGenerator) generateObjectDoc(element map[string]interface{}, name string, parentPath string) map[string]string {
	result := map[string]string{}
	properties := getElement(element, "properties")
	if properties == nil {
		return result
	}

	var requiredChildren []interface{}
	if rc := getElement(element, "required"); rc != nil {
		requiredChildren = rc.([]interface{})
	}

	propMap := properties.(map[string]interface{})
	for _, propName := range sortKeys(propMap) {
		propRequired := contains(requiredChildren, name)
		mergeMaps(result, generator.generateElementDoc(propMap[propName], propName, propRequired, parentPath+name+"."))
	}
	return result
}

func generateTableRow(fullName string, required bool, description string, name string) string {
	return fmt.Sprintf("| **%s** | %s | %s |",
		fullName, boolToRequiredLabel(required), normalizeDescription(description, name))
}

func getElement(obj interface{}, path ...string) interface{} {
	elem := obj
	for _, p := range path {
		elem = elem.(map[string]interface{})[p]
	}
	return elem
}

func normalizeDescription(description string, name string) any {
	description_trimmed := strings.Trim(description, " ")
	name_trimmed := strings.Trim(name, " ")
	if len(name_trimmed) == 0 {
		return description_trimmed
	}
	dParts := strings.SplitN(description_trimmed, " ", 2)
	if len(dParts) < 2 {
		return description
	}
	if !strings.EqualFold(name_trimmed, dParts[0]) {
		return description
	}
	description_trimmed = strings.Trim(dParts[1], " ")
	description_trimmed = strings.ToUpper(description_trimmed[:1]) + description_trimmed[1:]
	return description_trimmed
}

func pairsToParamsToSkip(toSkip map[string]bool, pairs [][]string, isToSkip bool) {
	for _, pair := range pairs {
		paramName := pair[1]
		toSkip[paramName] = isToSkip
	}
}

func sortKeys[T any](propMap map[string]T) []string {
	var keys []string
	for key := range propMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func boolToRequiredLabel(isRequired bool) string {
	if isRequired {
		return "Yes"
	}
	return "No"
}

func mergeMaps(dest map[string]string, src map[string]string) {
	for k, v := range src {
		dest[k] = v
	}
}

func contains(s []interface{}, e string) bool {
	for _, a := range s {
		if a.(string) == e {
			return true
		}
	}
	return false
}
