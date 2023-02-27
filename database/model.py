import random

from sqlalchemy import create_engine
from sqlalchemy import select
from sqlalchemy.orm import Session

from database.anek import Anek
from database.base import Base
from database.categories import Categories


class Model:

    def __init__(self):
        self.engine = create_engine("sqlite://", echo=True)
        self.session = Session(self.engine)
        Base.metadata.create_all(self.engine)

    def get_random_anek(self, anek_type):
        select_anek_category_id = select(Categories.id).where(Categories.name.is_(anek_type))
        category_id = self.session.scalars(select_anek_category_id).one()
        print(category_id)
        anek_ids = [anek_id for anek_id in self.session.scalars(select(Anek.id).where(Anek.category == category_id))]
        min_anek_id = min(anek_ids)
        max_anek_id = max(anek_ids)
        select_anek_by_type = select(Anek).where(Anek.category.is_(category_id)).where(
            Anek.id.is_(random.randint(min_anek_id, max_anek_id)))
        anek = self.session.scalars(select_anek_by_type).one().text
        print(anek)
        return anek.replace(r'\n', '\n')

    def create_categories_table(self):
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
                self.session.add(category)
            self.session.commit()

    def create_anek_table(self):
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
                self.session.add(anek)
            self.session.commit()

    def start_session(self):
        self.create_categories_table()
        self.create_anek_table()
