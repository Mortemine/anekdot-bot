import sqlite3
from sqlite3 import Error

connection = sqlite3.connect('aneks.db', check_same_thread=False)
cursor = connection.cursor()

def execute_select_query(query, connection=connection):
    try:
        data = cursor.execute(query)
        connection.commit()
        print('Query executed!')
        return data
    except Error:
        print(f'An error {Error} occured!')
        
