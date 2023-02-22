from sqlalchemy import create_engine
from sqlalchemy.orm import Session
from sqlalchemy import select
from base import Base
from categories import Categories
from anek import Anek
import random

engine = create_engine("sqlite://", echo=True)
Base.metadata.create_all(engine)

with Session(engine) as session:
    with open('../data/categories.txt', 'r', encoding='UTF-8') as f:
        categories = f.read().split('\n')
        for line in categories:
            stop = line.find(',')
            category_id = line[:stop]
            cat_start = line.find(',') + 2
            cat_text = line[cat_start:]
            category = Categories(
                id=category_id,
                name=cat_text
            )
            session.add(category)
        session.commit()

    with open('../data/anek_data.txt', 'r', encoding='UTF-8') as f:
        aneks = f.read().split('\n')
        for line in aneks:
            stop = line.find(',')
            try:
                category_id = line[1:stop]
            except IndexError:
                pass
            text_start = line.find(',') + 2
            anek_text = line[text_start:]
            anek = Anek(
                category=category_id,
                text=anek_text
            )
            session.add(anek)
        session.commit()
    anek_type = 34
    anek_ids = [anek_id for anek_id in session.scalars(select(Anek.id).where(Anek.category.is_(anek_type)))]
    min_anek_id = min(anek_ids)
    max_anek_id = max(anek_ids)
    stmt = select(Anek).where(Anek.category.is_(anek_type)).where(Anek.id.is_(random.randint(min_anek_id, max_anek_id)))

    for anek in session.scalars(stmt):
        print('\n')
        print(anek.text.replace(r'\n', '\n'))
        print('\n')
