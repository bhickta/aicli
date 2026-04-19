import json
from pathlib import Path
from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig
from aicli.services.analyze.segmenter import AnswerSegmenterService

db = AnalyzeDB(Path("./data/analyze.db"))
config = AnalyzeConfig()
# LMStudioProvider() takes no args, uses global config
provider = LMStudioProvider()
service = AnswerSegmenterService(provider, config)

pdf_file = "159.pdf"
pages = db.get_pages_for_pdf(pdf_file)
metadata = service._extract_metadata(pages, db)
print("Extracted Metadata:", metadata)

count = service.segment_pdf(pdf_file, db)
print("Segmented Answers Count:", count)
