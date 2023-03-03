from database import BASE
from sqlalchemy import Column, Integer, String


class Categories(BASE):
    __tablename__ = 'categories'

    cat_id = Column(Integer, primary_key=True)
    name = Column(String, nullable=False)

    def __repr__(self) -> str:
        return f'Categories(cat_id={self.cat_id!r}, name={self.name!r})'


Categories.__table__.create(checkfirst=True, bind=BASE.metadata.bind)
