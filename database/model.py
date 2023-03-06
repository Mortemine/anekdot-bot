import random


from database.categories import Categories
from database.anek import Anek
from database import SESSION


def get_random_anek(anek_type):
    category_id = SESSION.query(Categories.cat_id).filter(Categories.name == anek_type).one()[0]
    anek_ids = SESSION.query(Anek.anek_id).filter(Anek.category == category_id).all()
    min_anek_id = min(anek_ids)[0]
    max_anek_id = max(anek_ids)[0]
    random_id = random.randint(min_anek_id, max_anek_id)
    anek = SESSION.query(Anek.text).filter(Anek.category == category_id).filter(Anek.anek_id == random_id).one()[0]
    return anek.replace(r'\n', '\n')


def create_categories_table():
    with open('../data/categories.txt', 'r', encoding='UTF-8') as f:
        categories = f.read().split('\n')
        for line in categories:
            stop = line.find(',')
            category_id = line[:stop]
            cat_start = line.find(',') + 2
            cat_text = line[cat_start:]
            category = Categories(
                cat_id=category_id,
                name=cat_text
            )
            SESSION.add(category)
        SESSION.commit()


def create_anek_table():

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
            SESSION.add(anek)
        SESSION.commit()


def get_all_categories():
    categories = [name[0] for name in SESSION.query(Categories.name).order_by(Categories.name).all()]
    return categories


def start_session():
    create_categories_table()
    create_anek_table()
