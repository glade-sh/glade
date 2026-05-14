#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";

const repoRoot = path.resolve(new URL("..", import.meta.url).pathname);
const inputDir = process.argv[2] ? path.resolve(process.argv[2]) : path.join(repoRoot, "tmp", "standard-describes");
const outputFile = process.argv[3] ? path.resolve(process.argv[3]) : path.join(repoRoot, "internal", "storage", "standard_schema_generated.go");

const personAccountFields = new Set([
  "FirstName",
  "LastName",
  "MiddleName",
  "Suffix",
  "Salutation",
  "IsPersonAccount",
]);

function goString(value) {
  return JSON.stringify(value ?? "");
}

function goStringSlice(values) {
  if (!values || values.length === 0) return "nil";
  return `[]string{${values.map(goString).join(", ")}}`;
}

function fieldType(field) {
  if (field.calculated) return "FieldCalculated";
  switch (field.type) {
    case "id":
      return "FieldID";
    case "string":
    case "textarea":
    case "email":
    case "phone":
    case "url":
    case "combobox":
    case "encryptedstring":
      return "FieldString";
    case "picklist":
    case "multipicklist":
      return "FieldPicklist";
    case "boolean":
      return "FieldBoolean";
    case "int":
      return "FieldInteger";
    case "double":
    case "currency":
    case "percent":
      return "FieldDecimal";
    case "date":
      return "FieldDate";
    case "datetime":
      return "FieldDateTime";
    case "reference":
      return "FieldReference";
    case "base64":
      return "FieldBlob";
    case "address":
      return "FieldAddress";
    case "location":
      return "FieldLocation";
    default:
      return "FieldAny";
  }
}

function displayType(field) {
  switch (field.type) {
    case "id":
      return "ID";
    case "string":
      return "STRING";
    case "textarea":
      return "TEXTAREA";
    case "email":
      return "EMAIL";
    case "phone":
      return "PHONE";
    case "url":
      return "URL";
    case "picklist":
      return "PICKLIST";
    case "multipicklist":
      return "MULTIPICKLIST";
    case "boolean":
      return "BOOLEAN";
    case "int":
      return "INTEGER";
    case "double":
      return "DOUBLE";
    case "currency":
      return "CURRENCY";
    case "percent":
      return "PERCENT";
    case "date":
      return "DATE";
    case "datetime":
      return "DATETIME";
    case "reference":
      return "REFERENCE";
    case "base64":
      return "BLOB";
    case "address":
      return "ADDRESS";
    case "location":
      return "LOCATION";
    default:
      return "";
  }
}

function fieldFeature(objectName, field) {
  if (field.name === "CurrencyIsoCode") return "MultiCurrency";
  if (objectName === "Account" && (field.name.startsWith("Person") || personAccountFields.has(field.name))) {
    return "PersonAccounts";
  }
  return "";
}

function recordTypeFeature(objectName, recordType) {
  if (objectName === "Account" && recordType.developerName !== "Master") return "PersonAccounts";
  return "";
}

function fieldLiteral(field) {
  const pieces = [
    `APIName: ${goString(field.name)}`,
    `Label: ${goString(field.label || field.name)}`,
    `Type: ${fieldType(field)}`,
  ];
  const dt = displayType(field);
  if (dt) pieces.push(`DisplayType: ${goString(dt)}`);
  if (field.length) pieces.push(`Length: ${field.length}`);
  if (field.precision) pieces.push(`Precision: ${field.precision}`);
  if (field.scale) pieces.push(`Scale: ${field.scale}`);
  if (field.defaultValue !== null && field.defaultValue !== undefined) pieces.push(`DefaultValue: ${goString(String(field.defaultValue))}`);
  if (field.compoundFieldName) pieces.push(`CompoundFieldName: ${goString(field.compoundFieldName)}`);
  for (const [source, target] of [
    ["nillable", "Nillable"],
    ["defaultedOnCreate", "DefaultedOnCreate"],
    ["createable", "Createable"],
    ["updateable", "Updateable"],
    ["filterable", "Filterable"],
    ["groupable", "Groupable"],
    ["sortable", "Sortable"],
    ["aggregatable", "Aggregatable"],
    ["permissionable", "Permissionable"],
    ["deprecatedAndHidden", "DeprecatedAndHidden"],
  ]) {
    if (field[source] !== undefined && field[source] !== null) {
      pieces.push(`${target}: BoolFlag(${field[source] ? "true" : "false"})`);
    }
  }
  if (!field.nillable && field.createable && !field.defaultedOnCreate && field.defaultValue === null && field.defaultValueFormula === null) {
    pieces.push("Required: true");
  }
  if (field.externalId) pieces.push("ExternalID: true");
  if (field.unique) pieces.push("Unique: true");
  if (field.encrypted) pieces.push("Encrypted: true");
  if (field.caseSensitive) pieces.push("CaseSensitive: true");
  if (field.referenceTo?.length) pieces.push(`ReferenceTo: ${goStringSlice(field.referenceTo)}`);
  if (field.relationshipName) pieces.push(`RelationshipName: ${goString(field.relationshipName)}`);
  if (field.picklistValues?.length) {
    const values = field.picklistValues
      .filter((value) => value.value !== undefined && value.value !== null)
      .map((value) => {
        const parts = [`Value: ${goString(value.value)}`];
        if (value.label) parts.push(`Label: ${goString(value.label)}`);
        if (value.defaultValue) parts.push("Default: true");
        if (value.active) parts.push("Active: true");
        return `{${parts.join(", ")}}`;
      });
    if (values.length) pieces.push(`PicklistValues: []PicklistValue{${values.join(", ")}}`);
  }
  return `Field{${pieces.join(", ")}}`;
}

function childRelationshipKey(childObject, fieldName) {
  return `${(childObject || "").toLowerCase()}.${(fieldName || "").toLowerCase()}`;
}

function relationLiteral(field, childRelationship) {
  const relationshipName = field.relationshipName || (field.name.endsWith("Id") ? field.name.slice(0, -2) : "");
  const pieces = [
    `Field: ${goString(field.name)}`,
    `ParentObjects: ${goStringSlice(field.referenceTo)}`,
  ];
  if (relationshipName) pieces.push(`ParentRelationship: ${goString(relationshipName)}`);
  if (childRelationship?.relationshipName) pieces.push(`ChildRelationship: ${goString(childRelationship.relationshipName)}`);
  if (childRelationship?.cascadeDelete) pieces.push("CascadeDelete: true");
  if (childRelationship?.restrictedDelete) pieces.push("RestrictedDelete: true");
  if (field.referenceTo?.length > 1 || field.polymorphicForeignKey) pieces.push("Polymorphic: true");
  return `Relationship{${pieces.join(", ")}}`;
}

function recordTypeLiteral(recordType) {
  const pieces = [
    `ID: ${goString(recordType.recordTypeId || "")}`,
    `DeveloperName: ${goString(recordType.developerName)}`,
    `Name: ${goString(recordType.name || recordType.developerName)}`,
  ];
  if (recordType.active) pieces.push("Active: true");
  if (recordType.available) pieces.push("Available: true");
  if (recordType.defaultRecordTypeMapping) pieces.push("Default: true");
  return `RecordTypeInfo{${pieces.join(", ")}}`;
}

function mapLiteral(fields) {
  if (!fields.length) return "nil";
  return `map[string]Field{\n${fields.map((field) => `\t\t\t\t${goString(field.name)}: ${fieldLiteral(field)},`).join("\n")}\n\t\t\t}`;
}

function featureFieldsLiteral(groups) {
  const names = Object.keys(groups).sort();
  if (!names.length) return "nil";
  return `map[string]map[string]Field{\n${names.map((name) => `\t\t\t${goString(name)}: ${mapLiteral(groups[name])},`).join("\n")}\n\t\t}`;
}

function recordTypesLiteral(recordTypes) {
  if (!recordTypes.length) return "nil";
  return `[]RecordTypeInfo{${recordTypes.map(recordTypeLiteral).join(", ")}}`;
}

function featureRecordTypesLiteral(groups) {
  const names = Object.keys(groups).sort();
  if (!names.length) return "nil";
  return `map[string][]RecordTypeInfo{\n${names.map((name) => `\t\t\t${goString(name)}: ${recordTypesLiteral(groups[name])},`).join("\n")}\n\t\t}`;
}

const files = fs.readdirSync(inputDir).filter((file) => file.endsWith(".json")).sort();
const objects = files.map((file) => JSON.parse(fs.readFileSync(path.join(inputDir, file), "utf8")).result);
const childRelationships = new Map();
for (const obj of objects) {
  for (const relationship of obj.childRelationships || []) {
    if (!relationship.childSObject || !relationship.field || !relationship.relationshipName || relationship.deprecatedAndHidden) {
      continue;
    }
    const key = childRelationshipKey(relationship.childSObject, relationship.field);
    const existing = childRelationships.get(key);
    if (existing && existing.relationshipName !== relationship.relationshipName) {
      childRelationships.set(key, { conflict: true });
      continue;
    }
    childRelationships.set(key, {
      relationshipName: relationship.relationshipName,
      cascadeDelete: Boolean(existing?.cascadeDelete || relationship.cascadeDelete),
      restrictedDelete: Boolean(existing?.restrictedDelete || relationship.restrictedDelete),
    });
  }
}
const entries = [];
for (const obj of objects) {
  const baseFields = [];
  const featureFields = {};
  for (const field of [...obj.fields].sort((a, b) => a.name.localeCompare(b.name))) {
    const feature = fieldFeature(obj.name, field);
    if (feature) {
      featureFields[feature] ??= [];
      featureFields[feature].push(field);
    } else {
      baseFields.push(field);
    }
  }
  const relations = baseFields
    .filter((field) => field.referenceTo?.length)
    .map((field) => ({ field, childRelationship: childRelationships.get(childRelationshipKey(obj.name, field.name)) }));
  const baseRecordTypes = [];
  const featureRecordTypes = {};
  for (const recordType of [...(obj.recordTypeInfos || [])].sort((a, b) => a.developerName.localeCompare(b.developerName))) {
    const feature = recordTypeFeature(obj.name, recordType);
    if (feature) {
      featureRecordTypes[feature] ??= [];
      featureRecordTypes[feature].push(recordType);
    } else {
      baseRecordTypes.push(recordType);
    }
  }
  entries.push({ obj, baseFields, featureFields, relations, baseRecordTypes, featureRecordTypes });
}

let out = `// Code generated by scripts/generate-standard-schema.mjs; DO NOT EDIT.\n\npackage storage\n\ntype standardObjectCatalogEntry struct {\n\tDefinition         ObjectDefinition\n\tFeatureFields      map[string]map[string]Field\n\tFeatureRecordTypes map[string][]RecordTypeInfo\n}\n\nvar standardObjectKeyPrefixData = map[string]string{\n`;
for (const { obj } of entries) {
  if (obj.keyPrefix) out += `\t\t${goString(obj.name)}: ${goString(obj.keyPrefix)},\n`;
}
out += `}\n\nfunc standardObjectKeyPrefixes() map[string]string {\n\treturn standardObjectKeyPrefixData\n}\n\nfunc standardObjectCatalogEntryFor(objectName string) (standardObjectCatalogEntry, bool) {\n\tentry, ok := standardObjectCatalogData[objectName]\n\treturn entry, ok\n}\n\nvar standardObjectCatalogData = map[string]standardObjectCatalogEntry{\n`;

for (const entry of entries) {
  const { obj, baseFields, featureFields, relations, baseRecordTypes, featureRecordTypes } = entry;
  out += `\t\t${goString(obj.name)}: {\n\t\t\tDefinition: ObjectDefinition{\n\t\t\t\tAPIName: ${goString(obj.name)},\n\t\t\t\tLabel: ${goString(obj.label || obj.name)},\n\t\t\t\tPluralLabel: ${goString(obj.labelPlural || `${obj.label || obj.name}s`)},\n`;
  if (obj.keyPrefix) out += `\t\t\t\tKeyPrefix: ${goString(obj.keyPrefix)},\n`;
  out += `\t\t\t\tFields: ${mapLiteral(baseFields)},\n`;
  out += `\t\t\t\tRelations: []Relationship{${relations.map((relation) => relationLiteral(relation.field, relation.childRelationship)).join(", ")}},\n`;
  out += `\t\t\t\tRecordTypes: ${recordTypesLiteral(baseRecordTypes)},\n`;
  out += `\t\t\t},\n`;
  out += `\t\t\tFeatureFields: ${featureFieldsLiteral(featureFields)},\n`;
  out += `\t\t\tFeatureRecordTypes: ${featureRecordTypesLiteral(featureRecordTypes)},\n`;
  out += `\t\t},\n`;
}
out += `}\n`;

fs.writeFileSync(outputFile, out);
const gofmt = spawnSync("gofmt", ["-w", outputFile], { stdio: "inherit" });
if (gofmt.status !== 0) process.exit(gofmt.status ?? 1);
