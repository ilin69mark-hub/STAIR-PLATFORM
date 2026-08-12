# STAIR PLATFORM

Document: 08_DOMAIN_SERVICES.md

ID: DOM-0009

Status: APPROVED

---

# Purpose

Domain Service инкапсулирует бизнес-логику, которая не принадлежит одной Entity или Aggregate.

---

# Rules

Domain Service:

не хранит состояние;

не зависит от инфраструктуры;

детерминирован;

не содержит UI-логики.

---

# Registry

## Project

ProjectLifecycleService

RevisionService

---

## Geometry

GeometryBuilder

GeometryExporter

GeometryValidator

---

## Constraints

RuleEngine

ConstraintResolver

---

## Validation

ValidationEngine

IssueAnalyzer

---

## Solver

AnalysisEngine

LoadGenerator

---

## Optimization

Optimizer

RankingEngine

ScenarioGenerator

---

## Manufacturing

BOMBuilder

DrawingBuilder

NestingPlanner

CNCBuilder

---

## Pricing

PricingEngine

MarginCalculator

TaxResolver

---

## Documents

DocumentGenerator

TemplateRenderer

---

## AI

RecommendationEngine

PromptBuilder

KnowledgeRetriever

---

# Service Rules

Domain Services:

не используют SQL;

не знают HTTP;

не знают UI;

не знают ORM;

оперируют исключительно доменной моделью.

---

APPROVED