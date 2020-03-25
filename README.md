# tinystatic 

![logo.png](https://github.com/julvo/tinystatic/blob/master/logo.png "tinystatic logo")

A tiny static website generator that is flexible and easy to use. It's flexible, as there is no required website structure nor any blog-specific concepts. It's easy to use, as we can start with a standard HTML site and introduce tinystatic gradually.

The concept of tinystatic is simple: From every file in an input directory, create a file in an output directory which we can then use as the public directory of our webserver. How tinystatic generates an output file depends on the input file extension: Markdown is converted to HTML, while CSS, JS and images are simply copied. For markdown and HTML files, you can specify meta data at the top of a file. By specifying a template in this file meta data, and providing templates in separate directories, you can make use of Go's HTML templating engine. [Here](https://github.com/julvo/tinystatic/tree/master/examples/blog) an example of a typical blog website and for a quick start guide, see below.

## Install

### Pre-built binaries
Download the tinystatic binary for your operating system:
- [Linux](https://github.com/julvo/tinystatic/releases/download/v0.0.2/tinystatic_linux_amd64) 
- [macOS](https://github.com/julvo/tinystatic/releases/download/v0.0.2/tinystatic_macos_darwin_amd64) 

Optionally, add the binary to your shell path, by either placing the binary into an existing directory like `/usr/bin` or by adding the parent directory of the binary to your path variable.

If you added tinystatic to your path, you should be able to call
```shell
tinystatic -help
```
Otherwise, you will need to specify the path to the tinystatic binary when calling it
```shell
/path/to/tinystatic -help
```

### Compiling from source
If you don't want to use the pre-built binaries, you will need to install the Golang compiler to compile tinystatic. Then, you can install tinystatic by running
```shell
go install -u github.com/julvo/tinystatic
```
or by cloning the repository and running `go install` or `go build` in the root directory of this repository.

## Quick start
This is a 10-minute tutorial in which we build a small blog, starting with a single HTML page and introducing the features of tinystatic one-by-one.

First, we create a folder called `routes`. Inside this folder, we create a single HTML file `index.html` with the following contents:
```html
<!doctype html>
<html>
  <head>
    <title>Our first tinystatic website</title>
  </head>
  <body>
      <h1>Welcome to our blog</h1>
  </body>
</html>
```

Now, we can run `tinystatic` for the first time. By default, tinystatic expects to be called in the directory containing the `routes` directory, but you can change that by using the `-routes` parameter. After running the command, you should see a folder `output` appearing next to the `routes` folder. Our file structure now looks like this:
```
my-blog/
    routes/
        index.html
    output/
        index.html
```
We can now run a webserver in the output directory, e.g. using Python's built-in server to open our website on `http://localhost:8000`:
```
cd output
python3 -m http.server
```

So far, all tinystatic did was copying the `index.html` from `routes` to `output` - not all that useful, but hang in...

Let's add a second HTML file to `routes`, e.g. `about.html`:
```html
<!doctype html>
<html>
  <head>
    <title>Our first tinystatic website</title>
  </head>
  <body>
      <h1>About us</h1>
  </body>
</html>
```

After we run `tinystatic` again, and with our webserver still running, we can now navigate to `http://localhost:8000/about`. Note how there is no `.html` in this route anymore, as tinystatic created a folder `about` with a single `index.html` in it, like so:
```
output/
    index.html
    about/
        index.html
```

What we don't like about our current pages is the duplication of all the basic HTML structure. Wouldn't it be better to use a shared template for `index.html` and `about.html`?. To do this, we create a folder called `templates` next to our `routes` folder and place an HTML file `default.html` inside it: 

```
my-blog/
    routes/
        index.html
        about.html
    templates/
        default.html
```
The content of `default.html` should be:
```html
<!doctype html>
<html>
  <head>
    <title>Our first tinystatic website</title>
  </head>
  <body>
      {{template "body" .}}
  </body>
</html>
```

Also, we change the content of `routes/index.html` to
```html
---
template: default.html
---
{{define "body"}}
<h1>Welcome to our blog</h1>
{{end}}
```

and the content of `routes/about.html` to 
```html
---
template: default.html
---
{{define "body"}}
<h1>About us</h1>
{{end}}
```

When running `tinystatic` again, the output is identical to the previous output, but we consolidated the HTML skeleton into a single place.

As seen just now, we can specify a template to render our content into by providing a template name in the meta data at the top of a file. We can also include other templates (see below) and use Go's template pipelines. While rendering, we have access to the meta data defined at the top of the file, a struct `Route` with fields `Route.Href`, `Route.FilePath` and `Route.Meta` which is again a map of meta data defined at the top of the file. Moreover, we can access `Routes`, which is a slice (think: array for people new to Go) of all routes, which we will learn more about further down.  

Let's use this meta data together with Go's templating primitives to change the page title depending on the current page. For this, we change the meta data in `routes/about.html` to
```
---
template: default.html
title: About
---
```

and finally change `templates/default.html` to
```html
<!doctype html>
<html>
  <head>
    <title>{{if .title}} {{.title}} | {{end}}Our first tinystatic website</title>
  </head>
  <body>
      {{template "body" .}}
  </body>
</html>
```

After regenerating the website, the browser should now display different page titles for our index and our about page.

Now, let's create a few blog posts in our routes folder, e.g.
```
routes/
    index.html
    about.html
    posts/
        first_post.md
        second_post.md
```

Place some markdown inside these `.md` files with a meta data section at the top specifying the template as `default.html`, similar to how we specified the meta data in `routes/index.html` and `routes/about.html`. For `first_post.md`, this could look like this:
```markdown
---
template: default.html
title: First Post
---

# Here could be some fine content

```


Running `tinystatic` again to regenerate the output, we can now visit `http://localhost:8000/posts/first_post` and `http://localhost:8000/posts/second_post`. The markdown has been converted HTML and placed inside a template called `body` for us, so that it renders into the `{{template "body" .}}` placeholder in `templates/default.html`. Note how this is different to `.html` files, where we need to call `{{define "body"}} ... {{end}}` manually.

Next, let's create a listing of our posts by using the aforementioned `Routes` slice. We change the content of `routes/index.html` to:
```html
---
template: default.html
---
{{define "body"}}
<h1>Welcome to our blog</h1>

<ul>
{{range .Routes | filterFilePath "**/posts/*.md"}}
<li>
    <a href={{.Href}}>{{.title}}</a>
</li>
{{end}}
</ul>
```

After regenerating, we should see a list of our posts on the index page. The `Routes` slice provides a list of all routes which we can filter using pre-defined helper functions, e.g. 
- `.Routes | filterFilePath "**/posts/*.md"` to display all files ending on `.md` in any folder called posts
- `.Routes | sortAsc "title"` to sort routes based on the meta data field `title`
- `.Routes | limit 10` to get only the first 10 routes
- `.Routes | offset 3` to skip the first three routes
- `.Routes | filter "title" "*Post"` to filter based on the meta data field `title` matching the pattern `*Post`
- `.Routes | filterFileName "*.md"` to get all files ending on `*.md`
- `.Routes | filterHref "/*"` to get all top-level routes
- `.Routes | filterFilePath "**/posts/*.md" | sortDesc "title" | limit 10` to combine some of the above

Next, we would like to use a different layout for posts than for the other pages. The posts should have an image before the text, whereby we want to define the image URL in the post meta data. Therefore, we add a second template called `templates/post.html` with the following content:

```html
<!doctype html>
<html>
  <head>
    <title>{{if .title}} {{.title}} | {{end}}Our first tinystatic website</title>
  </head>
  <body>
      <img src={{.image}} />
      {{template "body" .}}
  </body>
</html>
```

We change the post meta data to
```
---
template: post.html
title: First Post
image: https://some-image.url
---
```
Regenerating the output should give us a beautiful image above our post. However, we also ended up with duplicated HTML code in our templates again. To improve that, we create another folder next to `routes` and `templates` called `partials`. Inside partials, we create a file called `head.html` with
```html
{{define "head"}}
<head>
  <title>{{if .title}} {{.title}} | {{end}}Our first tinystatic website</title>
</head>
{{end}}
```
and we replace `<head>...</head>` in our templates with `{{template "head" .}}`, like so
```html
<!doctype html>
<html>
  {{template "head" .}}
  <body>
      {{template "body" .}}
  </body>
</html>
```
Now we reduced the code replication between different templates to a minimum. We can use this `partials` directory to store all kinds of reoccuring components, e.g. navigation bars or footers.

Note that we don't actually need to structure the project using the folder names that we used in this tutorial. These folders names are merely the defaults, but can be changed using the respective command line arguments (see `tinystatic -help` for more).

There is a full example of a blog [here](https://github.com/julvo/tinystatic/tree/master/examples/blog).