#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = path.resolve(new URL("..", import.meta.url).pathname);
const localInputDir = path.join(repoRoot, "example-projects", "stubs", "apex-sobject-stubs");
const siblingInputDir = path.resolve(repoRoot, "..", "oaer", "example-projects", "stubs", "apex-sobject-stubs");
const defaultInputDir = fs.existsSync(localInputDir)
  ? localInputDir
  : fs.existsSync(siblingInputDir)
    ? siblingInputDir
    : path.join(repoRoot, "tmp", "apex-sobject-stubs");
const inputDir = process.argv[2] ? path.resolve(process.argv[2]) : defaultInputDir;
const outputFile = process.argv[3] ? path.resolve(process.argv[3]) : path.join(repoRoot, "internal", "storage", "standard_sobject_stub_overlay_generated.go");

function goString(value) {
  return JSON.stringify(value ?? "");
}

function goStringSlice(values) {
  if (!values || values.length === 0) return "nil";
  return `[]string{${values.map(goString).join(", ")}}`;
}

function cleanCommentLine(line) {
  return line
    .replace(/^\s*\/\*\*\s*/, "")
    .replace(/^\s*\*\s?/, "")
    .replace(/\s*\*\/\s*$/, "")
    .trim();
}

function displayType(apexType, fieldName) {
	const type = apexType.toLowerCase();
	if (fieldName === "Id") return "ID";
	if (fieldName.endsWith("Id")) return "REFERENCE";
  switch (type) {
    case "id":
      return "ID";
    case "boolean":
      return "BOOLEAN";
    case "integer":
    case "long":
      return "INTEGER";
    case "decimal":
    case "double":
      return "DOUBLE";
    case "date":
      return "DATE";
    case "datetime":
      return "DATETIME";
	case "blob":
		return "BLOB";
	case "address":
		return "ADDRESS";
	case "location":
		return "LOCATION";
	case "string":
		return "STRING";
	default:
		return "";
  }
}

function fieldType(apexType, fieldName) {
  const type = apexType.toLowerCase();
  if (fieldName === "Id") return "FieldID";
  if (fieldName.endsWith("Id")) return "FieldReference";
  switch (type) {
    case "id":
      return "FieldID";
    case "boolean":
      return "FieldBoolean";
    case "integer":
    case "long":
      return "FieldInteger";
    case "decimal":
    case "double":
      return "FieldDecimal";
    case "date":
      return "FieldDate";
    case "datetime":
      return "FieldDateTime";
	case "blob":
		return "FieldBlob";
	case "address":
		return "FieldAddress";
	case "location":
		return "FieldLocation";
	case "string":
		return "FieldString";
	default:
		return "FieldAny";
  }
}

function relationshipName(fieldName) {
	if (!fieldName.endsWith("Id") || fieldName === "Id") return "";
	return fieldName.slice(0, -2);
}

function shouldInferReferenceBreadth(field) {
	const type = field.apexType.toLowerCase();
	const concreteFieldName = `${type}id`;
	const relationship = (field.relationshipName || relationshipName(field.name)).toLowerCase();
	return field.name.toLowerCase() !== concreteFieldName && relationship !== type;
}

function isPersonAccountInferredParent(field, parentObject, relationshipNames) {
	if (parentObject.toLowerCase() !== "account") return false;
	if (field.name === "WhoId" || field.name.endsWith("ContactId")) return true;
	for (const relationshipName of relationshipNames || []) {
		if (relationshipName.startsWith("Person")) return true;
	}
	return false;
}

function fieldLiteral(field) {
	const pieces = [
		`APIName: ${goString(field.name)}`,
		`Label: ${goString(field.label || field.name)}`,
		`Type: ${fieldType(field.apexType, field.name)}`,
  ];
  const dt = displayType(field.apexType, field.name);
  if (dt) pieces.push(`DisplayType: ${goString(dt)}`);
	if (field.name.endsWith("Id") && field.name !== "Id" && field.referenceTo.length !== 0) {
		pieces.push(`ReferenceTo: ${goStringSlice(field.referenceTo)}`);
		const rel = field.relationshipName || relationshipName(field.name);
		if (rel) pieces.push(`RelationshipName: ${goString(rel)}`);
	}
	if (field.childRelationshipName) pieces.push(`ChildRelationshipName: ${goString(field.childRelationshipName)}`);
	return `Field{${pieces.join(", ")}}`;
}

function relationshipLiteral(relationship) {
	const pieces = [
		`Field: ${goString(relationship.fieldName)}`,
		`ParentObjects: ${goStringSlice([relationship.parentObject])}`,
	];
	const parentRelationship = relationship.parentRelationshipName || relationshipName(relationship.fieldName);
	if (parentRelationship) pieces.push(`ParentRelationship: ${goString(parentRelationship)}`);
	if (relationship.relationshipName) pieces.push(`ChildRelationship: ${goString(relationship.relationshipName)}`);
	if (relationship.polymorphic) pieces.push("Polymorphic: true");
	return `Relationship{${pieces.join(", ")}}`;
}

function listElementType(apexType) {
  const match = apexType.match(/^List<\s*([A-Za-z_][A-Za-z0-9_]*)\s*>$/);
  return match ? match[1] : "";
}

function parseSObjectStub(filePath) {
	const source = fs.readFileSync(filePath, "utf8");
	const classMatch = source.match(/global\s+class\s+([A-Za-z_][A-Za-z0-9_]*)\s+extends\s+SObject\b/);
	if (!classMatch) return null;
	const objectName = classMatch[1];
	const objectComment = source.match(/\/\*\*[\s\S]*?Schema object:\s*([^\n(]+)(?:\s*\(([^)\n]+)\))?[\s\S]*?Plural label:\s*([^\n*]+)[\s\S]*?\*\//);
	const label = objectComment ? (objectComment[2] || objectComment[1]).trim() : objectName;
	const pluralLabel = objectComment ? objectComment[3].trim() : `${objectName}s`;
	const fieldsBlockMatch = source.match(/global\s+class\s+SObjectFields\s*\{([\s\S]*?)^\s*\}/m);
	if (!fieldsBlockMatch) return null;
  const fieldNames = [];
  for (const match of fieldsBlockMatch[1].matchAll(/\bpublic\s+SObjectField\s+([A-Za-z_][A-Za-z0-9_]*)\s*;/g)) {
    fieldNames.push(match[1]);
  }
  if (fieldNames.length === 0) return null;

	const properties = new Map();
	const parentRelationships = new Map();
	const childRelationships = [];
	let comment = [];
	let childRelationshipField = "";
	let parentRelationshipField = "";
	let inComment = false;
	for (const line of source.split(/\r?\n/)) {
		const trimmed = line.trim();
		if (trimmed.startsWith("/**")) {
			inComment = true;
			comment = [];
			childRelationshipField = "";
			parentRelationshipField = "";
		}
		if (inComment) {
			const text = cleanCommentLine(line);
			const childMatch = text.match(/^Child relationship via ([A-Za-z_][A-Za-z0-9_]*)\b/);
			const parentMatch = text.match(/^Parent relationship for ([A-Za-z_][A-Za-z0-9_]*)\b/);
			if (childMatch) {
				childRelationshipField = childMatch[1];
			} else if (parentMatch) {
				parentRelationshipField = parentMatch[1];
			} else if (text && !text.startsWith("Parent relationship")) {
				comment.push(text);
			}
      if (trimmed.endsWith("*/")) {
        inComment = false;
      }
      continue;
    }
    const propertyMatch = line.match(/^\s*global\s+([A-Za-z_][A-Za-z0-9_.]*(?:<[^>{};]+>)?)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{\s*get;/);
	if (propertyMatch) {
		const childObject = listElementType(propertyMatch[1]);
		if (childRelationshipField && childObject) {
			childRelationships.push({
				parentObject: objectName,
				childObject,
				fieldName: childRelationshipField,
				relationshipName: propertyMatch[2],
			});
		} else if (parentRelationshipField) {
			parentRelationships.set(parentRelationshipField, propertyMatch[2]);
		}
		properties.set(propertyMatch[2], {
			apexType: propertyMatch[1],
			label: comment[0] || propertyMatch[2],
      });
      comment = [];
      childRelationshipField = "";
    }
  }

	const fields = [];
	for (const name of fieldNames) {
		const property = properties.get(name) || { apexType: "Object", label: name };
		const referenceTo = name.endsWith("Id") && name !== "Id" && property.apexType && !["Id", "String"].includes(property.apexType) ? [property.apexType] : [];
		fields.push({ name, apexType: property.apexType, label: property.label, relationshipName: parentRelationships.get(name) || "", referenceTo });
	}
	fields.sort((a, b) => a.name.localeCompare(b.name));
	return { objectName, label, pluralLabel, fields, childRelationships };
}

const files = fs.readdirSync(inputDir).filter((file) => file.endsWith(".cls")).sort();
const entries = [];
for (const file of files) {
  const entry = parseSObjectStub(path.join(inputDir, file));
  if (entry) entries.push(entry);
}
const childRelationshipsByObjectAndField = new Map();
const childRelationshipParentsByObjectAndField = new Map();
const childRelationshipNamesByObjectFieldAndParent = new Map();
for (const entry of entries) {
	for (const relationship of entry.childRelationships) {
		const key = `${relationship.childObject}.${relationship.fieldName}`;
		if (!childRelationshipsByObjectAndField.has(key)) {
			childRelationshipsByObjectAndField.set(key, new Set());
		}
		childRelationshipsByObjectAndField.get(key).add(relationship.relationshipName);
		if (!childRelationshipParentsByObjectAndField.has(key)) {
			childRelationshipParentsByObjectAndField.set(key, new Set());
		}
		childRelationshipParentsByObjectAndField.get(key).add(relationship.parentObject);
		const parentKey = `${key}.${relationship.parentObject}`;
		if (!childRelationshipNamesByObjectFieldAndParent.has(parentKey)) {
			childRelationshipNamesByObjectFieldAndParent.set(parentKey, new Set());
		}
		childRelationshipNamesByObjectFieldAndParent.get(parentKey).add(relationship.relationshipName);
	}
}

let out = `// Code generated by scripts/generate-sobject-stub-overlay.mjs; DO NOT EDIT.\n\npackage storage\n\ntype standardSObjectStubObjectInfo struct {\n\tLabel string\n\tPluralLabel string\n}\n\nvar standardSObjectStubObjectData = map[string]standardSObjectStubObjectInfo{\n`;
for (const entry of entries) {
	out += `\t${goString(entry.objectName)}: {Label: ${goString(entry.label)}, PluralLabel: ${goString(entry.pluralLabel)}},\n`;
}
out += `}\n\nfunc standardSObjectStubObjectInfoFor(objectName string) (standardSObjectStubObjectInfo, bool) {\n\tinfo, ok := standardSObjectStubObjectData[objectName]\n\treturn info, ok\n}\n\nvar standardSObjectStubFieldData = map[string]map[string]Field{\n`;
for (const entry of entries) {
	out += `\t${goString(entry.objectName)}: {\n`;
	for (const field of entry.fields) {
		const key = `${entry.objectName}.${field.name}`;
		const childRelationshipNames = childRelationshipsByObjectAndField.get(key);
		const parentObjects = childRelationshipParentsByObjectAndField.get(key);
		if (parentObjects && shouldInferReferenceBreadth(field)) {
			for (const parentObject of parentObjects) {
				if (isPersonAccountInferredParent(field, parentObject, childRelationshipNamesByObjectFieldAndParent.get(`${key}.${parentObject}`))) {
					continue;
				}
				if (!field.referenceTo.some((target) => target.toLowerCase() === parentObject.toLowerCase())) {
					field.referenceTo.push(parentObject);
				}
			}
			field.referenceTo.sort();
		}
		field.childRelationshipName = childRelationshipNames && childRelationshipNames.size === 1 ? [...childRelationshipNames][0] : "";
		out += `\t\t${goString(field.name)}: ${fieldLiteral(field)},\n`;
	}
  out += "\t},\n";
}
out += `}\n\nfunc standardSObjectStubFieldsFor(objectName string) (map[string]Field, bool) {\n\tfields, ok := standardSObjectStubFieldData[objectName]\n\treturn fields, ok\n}\n\nfunc standardSObjectStubNames() []string {\n\tnames := make([]string, 0, len(standardSObjectStubFieldData))\n\tfor name := range standardSObjectStubFieldData {\n\t\tnames = append(names, name)\n\t}\n\treturn names\n}\n`;

const relationshipsByObject = new Map();
for (const entry of entries) {
	for (const relationship of entry.childRelationships) {
		const key = `${relationship.childObject}.${relationship.fieldName}`;
		const childEntry = entries.find((candidate) => candidate.objectName === relationship.childObject);
		const childField = childEntry ? childEntry.fields.find((field) => field.name === relationship.fieldName) : null;
		relationship.parentRelationshipName = childField?.relationshipName || relationshipName(relationship.fieldName);
		relationship.polymorphic = (childRelationshipParentsByObjectAndField.get(key)?.size || 0) > 1;
		if (!relationshipsByObject.has(relationship.childObject)) {
			relationshipsByObject.set(relationship.childObject, []);
		}
    relationshipsByObject.get(relationship.childObject).push(relationship);
  }
}
for (const relationships of relationshipsByObject.values()) {
  relationships.sort((a, b) => {
    const field = a.fieldName.localeCompare(b.fieldName);
    if (field !== 0) return field;
    const child = a.relationshipName.localeCompare(b.relationshipName);
    if (child !== 0) return child;
    return a.parentObject.localeCompare(b.parentObject);
  });
}

out += `\nvar standardSObjectStubRelationshipData = map[string][]Relationship{\n`;
for (const objectName of [...relationshipsByObject.keys()].sort()) {
  out += `\t${goString(objectName)}: {\n`;
  for (const relationship of relationshipsByObject.get(objectName)) {
    out += `\t\t${relationshipLiteral(relationship)},\n`;
  }
  out += "\t},\n";
}
out += `}\n\nfunc standardSObjectStubRelationshipsFor(objectName string) ([]Relationship, bool) {\n\trelationships, ok := standardSObjectStubRelationshipData[objectName]\n\treturn relationships, ok\n}\n`;

fs.writeFileSync(outputFile, out);
const gofmt = spawnSync("gofmt", ["-w", outputFile], { stdio: "inherit" });
if (gofmt.status !== 0) process.exit(gofmt.status ?? 1);
