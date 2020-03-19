blogx is a simple blogging webapp

It has one "interesting" feature: responses always include all data for that
page in the response.  This means all javascript and css is in the html
response, but also images and videos (as a base64 datauri).  This should make
pages fast to render.

MIT-licensed

# Using

To use, create a empty config file:

	blogx config-describe >blogx.conf

Now edit the file, test the config:

	blogx config-test blogx.conf

Launch:

	blogx serve blogx.conf

And connect with your browser.
The bottom right of the page links to the admin pages.


# todo

- make it clear this isn't great code
- add filters to convert image to png or jpg. (make sure we keep white bg in jpg). also make filter to output an image including its alt text.
- make it possible to request external images. i should link inline images to it, so people can link to them.
- make the admin images page faster. converting images to inline all the time is too slow. need a local (on disk) cache probably.
- add a method to cleanup all static files, or regen pages.  in case the templates change.
- split code into more files
- write test code
- make it easier to specify times in the backend
- make sure minification works properly
