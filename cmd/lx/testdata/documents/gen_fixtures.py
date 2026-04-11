#!/usr/bin/env python3
"""Generate document test fixtures for document_test.go."""

import pathlib

out = pathlib.Path(__file__).parent

# --- PDF ---
from fpdf import FPDF

pdf = FPDF()
pdf.add_page()
pdf.set_font("Helvetica", size=12)
pdf.cell(0, 10, "Hello PDF World")
pdf.ln()
pdf.cell(0, 10, "Second line of text")
pdf.output(out / "sample.pdf")
print("wrote sample.pdf")

# --- DOCX ---
from docx import Document

doc = Document()
doc.add_paragraph("Hello DOCX World")
doc.add_paragraph("Second paragraph")
doc.save(out / "sample.docx")
print("wrote sample.docx")

# --- XLSX ---
from openpyxl import Workbook

wb = Workbook()
ws = wb.active
ws.title = "Sheet1"
ws["A1"] = "Name"
ws["B1"] = "Score"
ws["A2"] = "Alice"
ws["B2"] = 42
wb.save(out / "sample.xlsx")
print("wrote sample.xlsx")
