#!/usr/bin/env python3

import io
import zipfile
import pathlib

out = pathlib.Path(__file__).parent

from fpdf import FPDF

pdf = FPDF()
pdf.add_page()
pdf.set_font("Helvetica", size=12)
pdf.cell(0, 10, "Hello PDF World")
pdf.ln()
pdf.cell(0, 10, "Second line of text")
pdf.output(out / "sample.pdf")
print("wrote sample.pdf")

from docx import Document

doc = Document()
doc.add_paragraph("Hello DOCX World")
doc.add_paragraph("Second paragraph")
doc.save(out / "sample.docx")
print("wrote sample.docx")

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

content_xml = """\
<?xml version="1.0" encoding="UTF-8"?>
<office:document-content
    xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
    xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
  <office:body>
    <office:text>
      <text:p>Hello ODT World</text:p>
      <text:p>Second paragraph</text:p>
    </office:text>
  </office:body>
</office:document-content>
"""

manifest_xml = """\
<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0">
  <manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/>
  <manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>
</manifest:manifest>
"""

buf = io.BytesIO()
with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
    info = zipfile.ZipInfo("mimetype")
    info.compress_type = zipfile.ZIP_STORED
    zf.writestr(info, "application/vnd.oasis.opendocument.text")
    zf.writestr("META-INF/manifest.xml", manifest_xml)
    zf.writestr("content.xml", content_xml)

(out / "sample.odt").write_bytes(buf.getvalue())
print("wrote sample.odt")
