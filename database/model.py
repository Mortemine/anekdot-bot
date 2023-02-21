from sqlalchemy import create_engine
from sqlalchemy.orm import Session
from sqlalchemy import select
from base import Base
from categories import Categories
from anek import Anek

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
            anek_text = line[text_start:].replace(r'\n', '\n')
            anek = Anek(
                category=category_id,
                text=anek_text
            )
            session.add(anek)
        session.commit()

    stmt = select(Anek).where(Anek.id.is_(10))

    for anek in session.scalars(stmt):
        print(anek)
